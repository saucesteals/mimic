package mimic

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	utls "github.com/refraction-networking/utls"
	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/fhttp/http2"
)

type Transport struct {
	T  *http.Transport
	T2 *http2.Transport

	dialer *net.Dialer

	tlsSpec *utls.ClientHelloSpec
}

func (t *Transport) dialTls(ctx context.Context, network string, addr string) (net.Conn, error) {
	dialConn, err := t.dialer.DialContext(ctx, network, addr)

	if err != nil {
		return nil, err
	}

	name, _, err := net.SplitHostPort(addr)

	if err != nil {
		return nil, err
	}

	tlsConn := utls.UClient(dialConn, &utls.Config{ServerName: name}, utls.HelloCustom)

	if err = tlsConn.ApplyPreset(t.tlsSpec); err != nil {
		return nil, err
	}

	if err = tlsConn.Handshake(); err != nil {
		return nil, err
	}

	return tlsConn, nil
}

func (c *ClientSpec) NewTransport() *Transport {
	t := &Transport{
		T:  &http.Transport{},
		T2: &http2.Transport{},
		dialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		tlsSpec: c.ToTLSSpec(),
	}

	t.T.DialTLSContext = t.dialTls
	t.T2.DialTLS = func(network, addr string, _ *tls.Config) (net.Conn, error) {
		return t.dialTls(context.Background(), network, addr)
	}
	t.T2.Settings = c.H2Options.Settings
	t.T2.MaxHeaderListSize = c.H2Options.MaxHeaderListSize
	t.T2.InitialWindowSize = c.H2Options.MaxHeaderListSize
	t.T2.HeaderTableSize = c.H2Options.MaxHeaderListSize

	return t
}
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, ok := req.Header[http.PHeaderOrderKey]; ok {
		return t.T2.RoundTrip(req)
	}
	return t.T.RoundTrip(req)
}
