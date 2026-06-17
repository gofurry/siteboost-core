package upstream

import (
	"context"
	"net"
	"time"
)

type DirectDialer struct {
	dialer net.Dialer
}

func NewDirectDialer(timeout time.Duration) *DirectDialer {
	return &DirectDialer{
		dialer: net.Dialer{Timeout: timeout},
	}
}

func (d *DirectDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, address)
}
