// Package mimic applies browser profiles to stock net/http using an importable
// crypto/tls fork and small connection-level HTTP wire adapters.
package mimic

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"slices"
	"strings"

	tlsx "github.com/saucesteals/tlsx"

	"github.com/saucesteals/mimic/wire"
)

type requestKey struct{}

// Transport sends browser-profiled requests and manages idle connections.
// Implementations are safe for concurrent use and should be reused.
type Transport interface {
	http.RoundTripper
	CloseIdleConnections()
}

// Config describes a browser-profiled transport. NewTransport snapshots HTTP and
// TLS configuration. Profile must remain immutable for the transport's lifetime.
type Config struct {
	// Profile is required and supplies browser TLS and HTTP wire choices.
	Profile *Profile
	// HTTP supplies standard transport options. Nil uses http.DefaultTransport.
	// Existing connections are not shared, and the supplied transport is not modified.
	// TLSClientConfig applies only to HTTPS proxies, not origin connections.
	HTTP *http.Transport
	// TLS configures origin connections. Nil uses default TLS settings.
	// Profile owns ALPN and Compatibility.
	TLS *tlsx.Config
}

type transport struct {
	http    *http.Transport
	profile *Profile
}

// NewTransport validates configuration and constructs a reusable transport.
// It performs no network I/O. ClientHello factory errors are returned when dialing.
func NewTransport(options Config) (Transport, error) {
	if options.Profile == nil || options.Profile.ClientHello == nil || options.Profile.HTTP2 == nil {
		return nil, errors.New("mimic: incomplete profile")
	}
	base := options.HTTP
	if base == nil {
		var ok bool
		base, ok = http.DefaultTransport.(*http.Transport)
		if !ok || base == nil {
			return nil, errors.New("mimic: invalid default transport")
		}
	}
	if base.DialTLS != nil || base.DialTLSContext != nil {
		return nil, errors.New("mimic: custom TLS dialing conflicts with browser profiles")
	}
	tr := base.Clone()
	cfg := &tlsx.Config{}
	if options.TLS != nil {
		cfg = options.TLS.Clone()
	}
	if cfg.Compatibility != nil {
		return nil, errors.New("mimic: browser profiles own TLS compatibility")
	}
	if len(cfg.CipherSuites) > 0 || len(cfg.CurvePreferences) > 0 {
		return nil, errors.New("mimic: browser profiles select cipher suites and curves")
	}
	h2, err := options.Profile.HTTP2.nativeConfig()
	if err != nil {
		return nil, err
	}
	t := &transport{
		http:    tr,
		profile: options.Profile,
	}
	tr.TLSNextProto = nil
	tr.ForceAttemptHTTP2 = true
	tr.HTTP2 = &h2
	dial := dialer(tr.DialContext)
	if dial == nil {
		if legacy := tr.Dial; legacy != nil {
			dial = func(_ context.Context, network, addr string) (net.Conn, error) {
				return legacy(network, addr)
			}
		} else {
			dial = (&net.Dialer{}).DialContext
		}
	}
	wrap := func(c net.Conn, protocol string) net.Conn {
		return t.wire(c, protocol, h2.MaxEncoderHeaderTableSize, nil)
	}
	proxy := tr.Proxy
	tr.Proxy = func(req *http.Request) (*url.URL, error) {
		if req.URL.Scheme == "https" || proxy == nil {
			return nil, nil
		}
		return proxy(req)
	}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := dial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		protocol := "http/1.1"
		if tr.Protocols != nil && tr.Protocols.UnencryptedHTTP2() && !tr.Protocols.HTTP1() {
			protocol = "h2"
		}
		return wrap(c, protocol), nil
	}
	tr.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		req := ctx.Value(requestKey{}).(*http.Request)
		if req.URL.Scheme == "http" {
			raw, err := dial(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			c := proxyTLS(raw, tr.TLSClientConfig, addr)
			return handshaker(c, c.HandshakeContext, c.ConnectionState, tr, wrap), nil
		}
		var proxyURL *url.URL
		if proxy != nil {
			var err error
			proxyURL, err = proxy(req.WithContext(ctx))
			if err != nil {
				return nil, err
			}
		}
		raw, err := dialTunnel(ctx, dial, tr, proxyURL, addr)
		if err != nil {
			return nil, err
		}
		config := cfg.Clone()
		hello, err := t.profile.ClientHello()
		if err != nil {
			raw.Close()
			return nil, err
		}
		if hello == nil {
			raw.Close()
			return nil, errors.New("mimic: nil ClientHello profile")
		}
		localSettings, err := wire.ParseHTTP2Settings(hello.ApplicationSettings["h2"])
		if err != nil {
			raw.Close()
			return nil, err
		}
		for _, setting := range localSettings {
			if !slices.Contains(t.profile.HTTP2.Settings, setting) {
				raw.Close()
				return nil, errors.New("mimic: ALPS settings must match the HTTP/2 profile")
			}
		}
		var peerSettings []wire.Setting
		spec := *hello
		spec.PeerApplicationSettings = func(protocol string, data []byte) error {
			if protocol != "h2" {
				return errors.New("mimic: application settings require HTTP/2")
			}
			var err error
			peerSettings, err = wire.ParseHTTP2Settings(data)
			if err == nil && hello.PeerApplicationSettings != nil {
				err = hello.PeerApplicationSettings(protocol, data)
			}
			return err
		}
		config.CurvePreferences = slices.Clone(hello.Groups)
		config.Compatibility = spec.Compatibility
		// An absent PSK-modes extension is a no-ticket profile, not a request
		// to hide native resumption state after it has already been created.
		if !slices.ContainsFunc(hello.Extensions, func(e wire.ClientHelloExtension) bool {
			return e.ID == 45
		}) {
			config.SessionTicketsDisabled = true
		}
		if config.ServerName == "" {
			config.ServerName, _, err = net.SplitHostPort(addr)
			if err != nil {
				raw.Close()
				return nil, err
			}
		}
		config.NextProtos = []string{"h2", "http/1.1"}
		if tr.Protocols != nil {
			config.NextProtos = nil
			if tr.Protocols.HTTP2() {
				config.NextProtos = append(config.NextProtos, "h2")
			}
			if tr.Protocols.HTTP1() {
				config.NextProtos = append(config.NextProtos, "http/1.1")
			}
		}
		c := tlsx.Client(raw, config)
		return handshaker(c, c.HandshakeContext, func() tls.ConnectionState {
			return c.ConnectionState().Standard()
		}, tr, func(c net.Conn, protocol string) net.Conn {
			return t.wire(c, protocol, h2.MaxEncoderHeaderTableSize, peerSettings)
		}), nil
	}

	return t, nil
}

// RoundTrip implements http.RoundTripper without mutating the caller's request.
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("mimic: nil request")
	}
	if req.URL == nil {
		if req.Body != nil {
			req.Body.Close()
		}
		return nil, errors.New("mimic: nil request URL")
	}
	tr := t.http
	r := req.Clone(context.WithValue(req.Context(), requestKey{}, req))
	if r.Header == nil {
		r.Header = make(http.Header)
	}
	profileEncoding := false
defaults:
	for k, v := range t.profile.Headers {
		for present := range r.Header {
			if strings.EqualFold(present, k) {
				continue defaults
			}
		}
		if strings.EqualFold(k, "Accept-Encoding") {
			k = "Accept-Encoding"
			profileEncoding = true
		}
		r.Header[k] = append([]string(nil), v...)
	}
	// Resolve all defaults before negotiating compression. net/http uses the
	// canonical keys for these fields; caller headers still win case-insensitively.
	for k, v := range r.Header {
		canonical := ""
		switch {
		case strings.EqualFold(k, "Accept-Encoding"):
			canonical = "Accept-Encoding"
		case strings.EqualFold(k, "Range"):
			canonical = "Range"
		}
		if canonical != "" && k != canonical {
			r.Header[canonical] = append(r.Header[canonical], v...)
			delete(r.Header, k)
		}
	}
	decode := false
	if profileEncoding {
		if tr.DisableCompression || r.Method == http.MethodHead || r.Header.Get("Range") != "" {
			r.Header.Del("Accept-Encoding")
		} else {
			decode = r.Header.Get("Accept-Encoding") != ""
		}
	}
	if !tr.DisableKeepAlives && !r.Close && r.Header.Get("Connection") == "" {
		r.Header.Set("Connection", "keep-alive")
	}
	wire.PrepareHeaderOrder(r.Header)
	order := t.profile.orderedHeaders(r.Header.Get("Sec-Fetch-Mode"))
	r = r.WithContext(httptrace.WithClientTrace(r.Context(), &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			if c, ok := info.Conn.(interface{ Begin([]string) }); ok {
				c.Begin(order)
			}
		},
	}))
	resp, err := tr.RoundTrip(r)
	if resp != nil {
		resp.Request = req
		if decode && err == nil {
			decodeResponse(resp)
		}
	}

	return resp, err
}

// CloseIdleConnections closes idle connections in the stock transport.
func (t *transport) CloseIdleConnections() {
	t.http.CloseIdleConnections()
}

func (t *transport) wire(c net.Conn, protocol string, encoderLimit int, peerSettings []wire.Setting) net.Conn {
	if protocol != "h2" {
		return &wire.HTTP1Conn{Conn: c}
	}
	p := t.profile.HTTP2

	return wire.NewHTTP2(c, wire.HTTP2Config{
		Settings:              p.Settings,
		PeerSettings:          peerSettings,
		PseudoHeaderOrder:     p.PseudoHeaderOrder,
		Priority:              p.Priority,
		HeaderOrder:           t.profile.orderedHeaders,
		EncoderLimit:          encoderLimit,
		StreamWindowIncrement: p.streamWindowIncrement(),
	})
}
