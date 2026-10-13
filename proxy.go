package mimic

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

type dialer func(context.Context, string, string) (net.Conn, error)

func (d dialer) Dial(network, addr string) (net.Conn, error) {
	return d(context.Background(), network, addr)
}

func (d dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d(ctx, network, addr)
}

func proxyTLS(c net.Conn, config *tls.Config, addr string) *tls.Conn {
	cfg := &tls.Config{}
	if config != nil {
		cfg = config.Clone()
	}
	cfg.ServerName, _, _ = net.SplitHostPort(addr)
	cfg.NextProtos = []string{"http/1.1"}

	return tls.Client(c, cfg)
}

type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func dialTunnel(ctx context.Context, dial dialer, config *http.Transport, p *url.URL, addr string) (_ net.Conn, err error) {
	if p == nil {
		return dial(ctx, "tcp", addr)
	}
	if p.Scheme == "socks5" || p.Scheme == "socks5h" {
		d, err := proxy.FromURL(p, dial)
		if err != nil {
			return nil, err
		}
		return d.(proxy.ContextDialer).DialContext(ctx, "tcp", addr)
	}
	if p.Scheme != "http" && p.Scheme != "https" {
		return nil, fmt.Errorf("mimic: unsupported proxy scheme %q", p.Scheme)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	endpoint := p.Host
	if p.Port() == "" {
		port := "80"
		if p.Scheme == "https" {
			port = "443"
		}
		endpoint = net.JoinHostPort(p.Hostname(), port)
	}
	c, err := dial(ctx, "tcp", endpoint)
	if err != nil {
		return nil, &net.OpError{
			Op:  "proxyconnect",
			Net: "tcp",
			Err: err,
		}
	}
	defer func() {
		if err != nil {
			c.Close()
		}
	}()
	if p.Scheme == "https" {
		secure := proxyTLS(c, config.TLSClientConfig, endpoint)
		trace := httptrace.ContextClientTrace(ctx)
		if trace != nil && trace.TLSHandshakeStart != nil {
			trace.TLSHandshakeStart()
		}
		handshakeCtx, done := handshakeContext(ctx, config.TLSHandshakeTimeout)
		err = secure.HandshakeContext(handshakeCtx)
		done()
		if trace != nil && trace.TLSHandshakeDone != nil {
			trace.TLSHandshakeDone(secure.ConnectionState(), err)
		}
		if err != nil {
			return nil, err
		}
		c = secure
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = c.SetDeadline(time.Now())
		close(done)
	})
	defer func() {
		if !stop() {
			<-done
		}
		c.SetDeadline(time.Time{})
		if ctx.Err() != nil && err != nil {
			err = ctx.Err()
		}
	}()
	headers := config.ProxyConnectHeader
	if config.GetProxyConnectHeader != nil {
		headers, err = config.GetProxyConnectHeader(ctx, p, addr)
		if err != nil {
			return nil, err
		}
	}
	headers = headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	if p.User != nil {
		password, _ := p.User.Password()
		headers.Set(
			"Proxy-Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte(p.User.Username()+":"+password)),
		)
	}
	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: headers,
	}
	if err = req.Write(c); err != nil {
		return nil, err
	}
	limit := config.MaxResponseHeaderBytes
	if limit <= 0 {
		limit = 10 << 20
	}
	limited := &io.LimitedReader{
		R: c,
		N: limit,
	}
	reader := bufio.NewReader(limited)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		return nil, err
	}
	limited.N = 1<<63 - 1
	if config.OnProxyConnectResponse != nil {
		if err = config.OnProxyConnectResponse(ctx, p, req, resp); err != nil {
			return nil, err
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("mimic: proxy CONNECT: " + resp.Status)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}

	return &bufferedConn{
		Conn: c,
		r:    reader,
	}, nil
}
