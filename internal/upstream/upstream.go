package upstream

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type Resolver interface {
	Resolve(ctx context.Context, host string) ([]net.IP, error)
}

type Config struct {
	Type     string
	Address  string
	Username string
	Password string
	Timeout  time.Duration
}

func ConfigFromApp(cfg config.Config) Config {
	return Config{
		Type:     cfg.Upstream.Type,
		Address:  cfg.Upstream.Address,
		Username: cfg.Upstream.Username,
		Password: cfg.Upstream.Password,
		Timeout:  cfg.Proxy.DialTimeout.Std(),
	}
}

func NewDialer(cfg Config, resolver Resolver) (Dialer, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	switch cfg.Type {
	case "", config.UpstreamDirect:
		if resolver == nil {
			return nil, fmt.Errorf("resolver is required for direct upstream")
		}
		return NewDirectDialer(resolver, cfg.Timeout), nil
	case config.UpstreamHTTP:
		return NewHTTPDialer(cfg)
	case config.UpstreamSOCKS5:
		return NewSOCKS5Dialer(cfg)
	default:
		return nil, fmt.Errorf("unsupported upstream type %q", cfg.Type)
	}
}

type DirectDialer struct {
	resolver Resolver
	dialer   net.Dialer
}

type DirectDialAttempt struct {
	Stage   string
	IP      net.IP
	Address string
	Err     error
}

func (a DirectDialAttempt) Error() string {
	stage := a.Stage
	if stage == "" {
		stage = "tcp"
	}
	if a.Err == nil {
		return fmt.Sprintf("%s %s failed", stage, a.Address)
	}
	return fmt.Sprintf("%s %s failed: %v", stage, a.Address, a.Err)
}

type DirectDialError struct {
	Network    string
	Host       string
	Port       string
	ResolveErr error
	Attempts   []DirectDialAttempt
}

func (e *DirectDialError) Error() string {
	target := net.JoinHostPort(e.Host, e.Port)
	if e.ResolveErr != nil {
		return fmt.Sprintf("direct upstream resolve %s failed: %v", target, e.ResolveErr)
	}
	if len(e.Attempts) == 0 {
		return fmt.Sprintf("direct upstream dial %s failed: no target IPs", target)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "direct upstream dial %s failed after %d attempt(s): ", target, len(e.Attempts))
	limit := len(e.Attempts)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(e.Attempts[i].Error())
	}
	if remaining := len(e.Attempts) - limit; remaining > 0 {
		fmt.Fprintf(&b, "; %d more attempt(s) omitted", remaining)
	}
	return b.String()
}

func (e *DirectDialError) Unwrap() error {
	if e.ResolveErr != nil {
		return e.ResolveErr
	}
	if len(e.Attempts) == 1 {
		return e.Attempts[0].Err
	}
	return nil
}

func NewDirectDialer(resolver Resolver, timeout time.Duration) *DirectDialer {
	return &DirectDialer{
		resolver: resolver,
		dialer:   net.Dialer{Timeout: timeout},
	}
}

func (d *DirectDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("split target address: %w", err)
	}
	ips, err := d.resolver.Resolve(ctx, host)
	if err != nil {
		return nil, &DirectDialError{Network: network, Host: host, Port: port, ResolveErr: err}
	}

	attempts := make([]DirectDialAttempt, 0, len(ips))
	for _, ip := range ips {
		target := net.JoinHostPort(ip.String(), port)
		conn, err := d.dialer.DialContext(ctx, network, target)
		if err == nil {
			return conn, nil
		}
		attempts = append(attempts, DirectDialAttempt{
			Stage:   "tcp",
			IP:      cloneIP(ip),
			Address: target,
			Err:     err,
		})
	}
	return nil, &DirectDialError{Network: network, Host: host, Port: port, Attempts: attempts}
}

func (d *DirectDialer) DialTLSContext(ctx context.Context, network, address string, tlsConfig *tls.Config) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("split target address: %w", err)
	}
	ips, err := d.resolver.Resolve(ctx, host)
	if err != nil {
		return nil, &DirectDialError{Network: network, Host: host, Port: port, ResolveErr: err}
	}

	attempts := make([]DirectDialAttempt, 0, len(ips))
	for _, ip := range ips {
		target := net.JoinHostPort(ip.String(), port)
		conn, err := d.dialer.DialContext(ctx, network, target)
		if err != nil {
			attempts = append(attempts, DirectDialAttempt{
				Stage:   "tcp",
				IP:      cloneIP(ip),
				Address: target,
				Err:     err,
			})
			continue
		}

		handshakeCtx, cancel := d.handshakeContext(ctx)
		tlsConn := tls.Client(conn, cloneTLSConfig(tlsConfig, host))
		if err := tlsConn.HandshakeContext(handshakeCtx); err != nil {
			cancel()
			_ = tlsConn.Close()
			attempts = append(attempts, DirectDialAttempt{
				Stage:   "tls",
				IP:      cloneIP(ip),
				Address: target,
				Err:     err,
			})
			continue
		}
		cancel()
		return tlsConn, nil
	}
	return nil, &DirectDialError{Network: network, Host: host, Port: port, Attempts: attempts}
}

func (d *DirectDialer) handshakeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok || d.dialer.Timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d.dialer.Timeout)
}

type HTTPDialer struct {
	proxyAddr string
	username  string
	password  string
	dialer    net.Dialer
	timeout   time.Duration
}

func NewHTTPDialer(cfg Config) (*HTTPDialer, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	addr, username, password, err := parseProxyAddress(cfg.Address, "http", cfg.Username, cfg.Password)
	if err != nil {
		return nil, err
	}
	return &HTTPDialer{
		proxyAddr: addr,
		username:  username,
		password:  password,
		dialer:    net.Dialer{Timeout: cfg.Timeout},
		timeout:   cfg.Timeout,
	}, nil
}

func (d *HTTPDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("HTTP upstream only supports TCP networks")
	}
	conn, err := d.dialer.DialContext(ctx, "tcp", d.proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("dial HTTP upstream: %w", err)
	}
	if err := setHandshakeDeadline(ctx, conn, d.timeout); err != nil {
		_ = conn.Close()
		return nil, err
	}

	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: address},
		Host:   address,
		Header: make(http.Header),
	}
	if d.username != "" || d.password != "" {
		token := base64.StdEncoding.EncodeToString([]byte(d.username + ":" + d.password))
		req.Header.Set("Proxy-Authorization", "Basic "+token)
	}
	if err := req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write HTTP CONNECT: %w", err)
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read HTTP CONNECT response: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP upstream CONNECT failed: %s", resp.Status)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

type SOCKS5Dialer struct {
	proxyAddr string
	username  string
	password  string
	dialer    net.Dialer
	timeout   time.Duration
}

func NewSOCKS5Dialer(cfg Config) (*SOCKS5Dialer, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	addr, username, password, err := parseProxyAddress(cfg.Address, "socks5", cfg.Username, cfg.Password)
	if err != nil {
		return nil, err
	}
	return &SOCKS5Dialer{
		proxyAddr: addr,
		username:  username,
		password:  password,
		dialer:    net.Dialer{Timeout: cfg.Timeout},
		timeout:   cfg.Timeout,
	}, nil
}

func (d *SOCKS5Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("SOCKS5 upstream only supports TCP networks")
	}
	conn, err := d.dialer.DialContext(ctx, "tcp", d.proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("dial SOCKS5 upstream: %w", err)
	}
	if err := setHandshakeDeadline(ctx, conn, d.timeout); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := d.handshake(conn, address); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (d *SOCKS5Dialer) handshake(conn net.Conn, target string) error {
	authRequired := d.username != "" || d.password != ""
	if authRequired {
		if _, err := conn.Write([]byte{0x05, 0x02, 0x00, 0x02}); err != nil {
			return fmt.Errorf("write SOCKS5 greeting: %w", err)
		}
	} else if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return fmt.Errorf("write SOCKS5 greeting: %w", err)
	}

	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil {
		return fmt.Errorf("read SOCKS5 greeting: %w", err)
	}
	if greeting[0] != 0x05 {
		return fmt.Errorf("invalid SOCKS5 version %d", greeting[0])
	}
	switch greeting[1] {
	case 0x00:
	case 0x02:
		if err := d.authenticate(conn); err != nil {
			return err
		}
	default:
		return fmt.Errorf("SOCKS5 upstream rejected authentication method")
	}

	request, err := socks5ConnectRequest(target)
	if err != nil {
		return err
	}
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("write SOCKS5 connect: %w", err)
	}
	if err := readSOCKS5Reply(conn); err != nil {
		return err
	}
	return nil
}

func (d *SOCKS5Dialer) authenticate(conn net.Conn) error {
	if len(d.username) > 255 || len(d.password) > 255 {
		return fmt.Errorf("SOCKS5 username/password are too long")
	}
	req := []byte{0x01, byte(len(d.username))}
	req = append(req, []byte(d.username)...)
	req = append(req, byte(len(d.password)))
	req = append(req, []byte(d.password)...)
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("write SOCKS5 auth: %w", err)
	}
	var resp [2]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		return fmt.Errorf("read SOCKS5 auth: %w", err)
	}
	if resp[0] != 0x01 || resp[1] != 0x00 {
		return fmt.Errorf("SOCKS5 authentication failed")
	}
	return nil
}

func socks5ConnectRequest(target string) ([]byte, error) {
	host, portString, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("split SOCKS5 target: %w", err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid SOCKS5 target port")
	}

	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			req = append(req, 0x01)
			req = append(req, ipv4...)
		} else {
			req = append(req, 0x04)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return nil, fmt.Errorf("SOCKS5 target host is too long")
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, []byte(host)...)
	}
	req = append(req, byte(port>>8), byte(port))
	return req, nil
}

func readSOCKS5Reply(conn net.Conn) error {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return fmt.Errorf("read SOCKS5 reply: %w", err)
	}
	if header[0] != 0x05 {
		return fmt.Errorf("invalid SOCKS5 reply version %d", header[0])
	}
	if header[1] != 0x00 {
		return fmt.Errorf("SOCKS5 connect failed with code %d", header[1])
	}
	var discard int
	switch header[3] {
	case 0x01:
		discard = net.IPv4len
	case 0x03:
		var length [1]byte
		if _, err := io.ReadFull(conn, length[:]); err != nil {
			return fmt.Errorf("read SOCKS5 domain length: %w", err)
		}
		discard = int(length[0])
	case 0x04:
		discard = net.IPv6len
	default:
		return fmt.Errorf("invalid SOCKS5 address type %d", header[3])
	}
	if discard > 0 {
		if _, err := io.ReadFull(conn, make([]byte, discard)); err != nil {
			return fmt.Errorf("read SOCKS5 bind address: %w", err)
		}
	}
	var port [2]byte
	if _, err := io.ReadFull(conn, port[:]); err != nil {
		return fmt.Errorf("read SOCKS5 bind port: %w", err)
	}
	return nil
}

func parseProxyAddress(address, defaultScheme, username, password string) (string, string, string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", "", "", fmt.Errorf("upstream address is required")
	}
	if !strings.Contains(address, "://") {
		address = defaultScheme + "://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil {
		return "", "", "", fmt.Errorf("parse upstream address: invalid URL")
	}
	if parsed.Host == "" {
		return "", "", "", fmt.Errorf("upstream address host is required")
	}
	if parsed.User != nil {
		if username == "" {
			username = parsed.User.Username()
		}
		if password == "" {
			if parsedPassword, ok := parsed.User.Password(); ok {
				password = parsedPassword
			}
		}
	}
	host := parsed.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, defaultPortForScheme(defaultScheme))
	}
	return host, username, password, nil
}

func defaultPortForScheme(scheme string) string {
	switch scheme {
	case "http":
		return "8080"
	case "socks5":
		return "1080"
	default:
		return "0"
	}
}

func setHandshakeDeadline(ctx context.Context, conn net.Conn, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set upstream deadline: %w", err)
	}
	return nil
}

func cloneIP(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	return append(net.IP(nil), ip...)
}

func cloneTLSConfig(cfg *tls.Config, serverName string) *tls.Config {
	var cloned *tls.Config
	if cfg == nil {
		cloned = &tls.Config{}
	} else {
		cloned = cfg.Clone()
	}
	if cloned.ServerName == "" {
		cloned.ServerName = serverName
	}
	return cloned
}
