package mimic

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

type browserConn struct {
	net.Conn
	handshake func(context.Context) error
	state     func() tls.ConnectionState
	wrap      func(net.Conn, string) net.Conn
	timeout   time.Duration
	http1     bool
	wrapOnce  sync.Once
}

func (c *browserConn) ConnectionState() tls.ConnectionState {
	return c.state()
}

func (c *browserConn) Begin(order []string) {
	if h, ok := c.Conn.(interface{ Begin([]string) }); ok {
		h.Begin(order)
	}
}

func (c *browserConn) HandshakeContext(ctx context.Context) error {
	ctx, cancel := handshakeContext(ctx, c.timeout)
	defer cancel()
	if err := c.handshake(ctx); err != nil {
		return err
	}
	protocol := c.state().NegotiatedProtocol
	if protocol != "h2" && !c.http1 {
		return errors.New("mimic: server did not negotiate required HTTP/2")
	}
	c.wrapOnce.Do(func() { c.Conn = c.wrap(c.Conn, protocol) })

	return nil
}

func handshaker(
	c net.Conn,
	handshake func(context.Context) error,
	state func() tls.ConnectionState,
	tr *http.Transport,
	wrap func(net.Conn, string) net.Conn,
) net.Conn {
	return &browserConn{
		Conn:      c,
		handshake: handshake,
		state:     state,
		timeout:   tr.TLSHandshakeTimeout,
		http1:     tr.Protocols == nil || tr.Protocols.HTTP1(),
		wrap:      wrap,
	}
}

func handshakeContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}

	return context.WithCancel(ctx)
}
