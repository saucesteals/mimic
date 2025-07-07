package mimic

import (
	"fmt"
	"math/rand/v2"
	"net"
	"time"

	http "github.com/saucesteals/fhttp"
)

type TransportOptions struct {
	// Version is the Chromium version of the browser to mimic
	Version string

	// Brand is the brand of the browser to mimic
	Brand Brand

	// Platform is the platform of the browser to mimic
	Platform Platform

	// Transport is the transport to use for the browser
	// If nil, a new http.Transport will be created
	Transport *http.Transport
}

// NewTransport creates a new Transport for the given options
// Given a Chromium browser's brand, version and platform, it will create a new Transport that mimics the following:
// - Pseudo Header Order
// - Sec-Ch-Ua
// - Sec-Ch-Ua-Mobile
// - Sec-Ch-Ua-Platform
// - User-Agent
//
// See Transport for more details.
func NewTransport(opts TransportOptions) (*Transport, error) {
	spec, err := Chromium(opts.Brand, opts.Version)
	if err != nil {
		return nil, err
	}

	if opts.Transport == nil {
		opts.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}

	header := http.Header{}
	header.Set("sec-ch-ua", spec.ClientHintUA())
	header.Set("sec-ch-ua-mobile", "?0")

	var platformAgentString string
	var platformHintString string
	switch opts.Platform {
	case PlatformWindows:
		platformAgentString = "Windows NT 10.0; Win64; x64"
		platformHintString = "Windows"
	case PlatformMac:
		platformAgentString = "Macintosh; Intel Mac OS X 10_15_7"
		platformHintString = "macOS"
	case PlatformLinux:
		platformAgentString = "X11; Linux x86_64"
		platformHintString = "Linux"
	default:
		return nil, fmt.Errorf("mimic: unsupported platform: %s", opts.Platform)
	}

	header.Set("user-agent", fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", platformAgentString, spec.Version()))
	header.Set("sec-ch-ua-platform", fmt.Sprintf(`"%s"`, platformHintString))

	return &Transport{
		Transport:         spec.ConfigureTransport(opts.Transport),
		PseudoHeaderOrder: spec.PseudoHeaderOrder(),
		DefaultHeaders:    header,
	}, nil
}

// Transport is a Transport that takes care of:
// - Randomizing the header order (if not set)
// - Pseudo Header Order
// - Default Headers. See NewTransport for its default headers.
type Transport struct {
	Transport         http.RoundTripper
	PseudoHeaderOrder []string
	DefaultHeaders    http.Header
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	header := req.Header

	header[http.PHeaderOrderKey] = t.PseudoHeaderOrder
	for key, value := range t.DefaultHeaders {
		if override := header.Get(key); override != "" {
			continue
		}

		if len(value) > 0 {
			header.Set(key, value[0])
		}
	}

	if header[http.HeaderOrderKey] == nil {
		var keys []string
		for key := range header {
			keys = append(keys, key)
		}

		rand.Shuffle(len(keys), func(i, j int) {
			keys[i], keys[j] = keys[j], keys[i]
		})

		header[http.HeaderOrderKey] = keys
	}

	return t.Transport.RoundTrip(req)
}
