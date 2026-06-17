package upstream

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeResolver struct {
	ips []net.IP
	err error
}

func (r fakeResolver) Resolve(ctx context.Context, host string) ([]net.IP, error) {
	return r.ips, r.err
}

func TestDirectDialerUsesResolver(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := echoOnce(t, ln)

	dialer := NewDirectDialer(fakeResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}}, 5*time.Second)
	conn, err := dialer.DialContext(context.Background(), "tcp", net.JoinHostPort("example.test", portOf(ln.Addr().String())))
	if err != nil {
		t.Fatal(err)
	}
	assertTunnel(t, conn)
	<-done
}

func TestHTTPDialerConnectsWithBasicAuth(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		req, err := http.ReadRequest(reader)
		if err != nil {
			done <- "read request: " + err.Error()
			return
		}
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		if req.Method != http.MethodConnect || req.Host != "example.test:443" {
			done <- fmt.Sprintf("bad CONNECT request: %s %s", req.Method, req.Host)
			return
		}
		if req.Header.Get("Proxy-Authorization") != wantAuth {
			done <- "bad auth"
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		buf := make([]byte, 4)
		if _, err := io.ReadFull(reader, buf); err != nil {
			done <- "read tunnel: " + err.Error()
			return
		}
		_, _ = conn.Write([]byte("pong"))
		done <- string(buf)
	}()

	dialer, err := NewHTTPDialer(Config{
		Address:  ln.Addr().String(),
		Username: "user",
		Password: "pass",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "example.test:443")
	if err != nil {
		t.Fatal(err)
	}
	assertTunnel(t, conn)
	if got := <-done; got != "ping" {
		t.Fatalf("server got %q", got)
	}
}

func TestSOCKS5DialerNoAuthUsesDomainTarget(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := socks5Once(t, ln, false, "example.test")

	dialer, err := NewSOCKS5Dialer(Config{Address: ln.Addr().String(), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "example.test:443")
	if err != nil {
		t.Fatal(err)
	}
	assertTunnel(t, conn)
	if got := <-done; got != "ping" {
		t.Fatalf("server got %q", got)
	}
}

func TestSOCKS5DialerUsernamePassword(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := socks5Once(t, ln, true, "example.test")

	dialer, err := NewSOCKS5Dialer(Config{
		Address:  ln.Addr().String(),
		Username: "user",
		Password: "pass",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "example.test:443")
	if err != nil {
		t.Fatal(err)
	}
	assertTunnel(t, conn)
	if got := <-done; got != "ping" {
		t.Fatalf("server got %q", got)
	}
}

func TestProxyAddressParseErrorDoesNotLeakPassword(t *testing.T) {
	_, err := NewHTTPDialer(Config{
		Address:  "http://user:secret@\n",
		Password: "another-secret",
		Timeout:  time.Second,
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	msg := err.Error()
	for _, secret := range []string{"secret", "another-secret"} {
		if strings.Contains(msg, secret) {
			t.Fatalf("error leaked password %q: %s", secret, msg)
		}
	}
}

func echoOnce(t *testing.T, ln net.Listener) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		_, _ = conn.Write([]byte("pong"))
	}()
	return done
}

func socks5Once(t *testing.T, ln net.Listener, auth bool, wantHost string) <-chan string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)

		header := make([]byte, 2)
		if _, err := io.ReadFull(reader, header); err != nil {
			done <- err.Error()
			return
		}
		methods := make([]byte, int(header[1]))
		if _, err := io.ReadFull(reader, methods); err != nil {
			done <- err.Error()
			return
		}
		if auth {
			_, _ = conn.Write([]byte{0x05, 0x02})
			authReq := make([]byte, 2)
			if _, err := io.ReadFull(reader, authReq); err != nil {
				done <- err.Error()
				return
			}
			username := make([]byte, int(authReq[1]))
			if _, err := io.ReadFull(reader, username); err != nil {
				done <- err.Error()
				return
			}
			passLen, err := reader.ReadByte()
			if err != nil {
				done <- err.Error()
				return
			}
			password := make([]byte, int(passLen))
			if _, err := io.ReadFull(reader, password); err != nil {
				done <- err.Error()
				return
			}
			if string(username) != "user" || string(password) != "pass" {
				done <- "bad credentials"
				return
			}
			_, _ = conn.Write([]byte{0x01, 0x00})
		} else {
			_, _ = conn.Write([]byte{0x05, 0x00})
		}

		req := make([]byte, 5)
		if _, err := io.ReadFull(reader, req); err != nil {
			done <- err.Error()
			return
		}
		if req[0] != 0x05 || req[1] != 0x01 || req[3] != 0x03 {
			done <- fmt.Sprintf("bad request header: %v", req[:4])
			return
		}
		hostLen := int(req[4])
		host := make([]byte, hostLen)
		if _, err := io.ReadFull(reader, host); err != nil {
			done <- err.Error()
			return
		}
		port := make([]byte, 2)
		if _, err := io.ReadFull(reader, port); err != nil {
			done <- err.Error()
			return
		}
		if string(host) != wantHost {
			done <- "bad host: " + string(host)
			return
		}
		_, _ = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0})
		buf := make([]byte, 4)
		if _, err := io.ReadFull(reader, buf); err != nil {
			done <- err.Error()
			return
		}
		_, _ = conn.Write([]byte("pong"))
		done <- string(buf)
	}()
	return done
}

func assertTunnel(t *testing.T, conn net.Conn) {
	t.Helper()
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 4)
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "pong" {
		t.Fatalf("got %q, want pong", got)
	}
}

func portOf(addr string) string {
	parts := strings.Split(addr, ":")
	return parts[len(parts)-1]
}
