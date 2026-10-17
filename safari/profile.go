// Package safari constructs Safari 26 TLS and HTTP/2 profiles.
package safari

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/saucesteals/mimic"
	"github.com/saucesteals/mimic/wire"
)

// Platform selects Safari's TLS profile and default User-Agent.
// IPad uses desktop browsing headers with the mobile TLS profile.
type Platform string

const (
	Mac    Platform = "mac"
	IPhone Platform = "iphone"
	IPad   Platform = "ipad"
)

// Config selects a Safari profile. All fields are required.
type Config struct {
	Version  string
	Platform Platform
}

// ErrUnsupportedVersion identifies versions without a supported snapshot.
var ErrUnsupportedVersion = errors.New("safari: unsupported version")

// New returns a Safari 26.0–26.3 profile, including the 26.0.1 TLS update.
// HTTP/2 uses the Safari 26 settings shared by the supported platforms.
// Headers and wire settings may be customized before creating a transport.
func New(config Config) (*mimic.Profile, error) {
	version, platform := config.Version, config.Platform
	switch version {
	case "26", "26.0", "26.0.1", "26.1", "26.2", "26.3":
	default:
		return nil, ErrUnsupportedVersion
	}
	if version == "26" {
		version = "26.0"
	}
	agent := "Macintosh; Intel Mac OS X 10_15_7"
	tail := "Safari/605.1.15"
	switch platform {
	case Mac, IPad:
	case IPhone:
		agent = "iPhone; CPU iPhone OS 18_7 like Mac OS X"
		if version == "26.0" {
			agent = "iPhone; CPU iPhone OS 18_6 like Mac OS X"
		}
		tail = "Mobile/15E148 Safari/604.1"
	default:
		return nil, fmt.Errorf("safari: unsupported platform: %s", platform)
	}
	acceptEncoding := "gzip, deflate, br"
	if version == "26.3" {
		// This snapshot assumes the matching OS networking stack.
		acceptEncoding += ", zstd"
	}
	return &mimic.Profile{
		ClientHello: func() (*wire.ClientHelloProfile, error) { return clientHello(version, platform) },
		HTTP2: &mimic.HTTP2Profile{
			Settings: []mimic.HTTP2Setting{
				{ID: 2, Val: 0},
				{ID: 3, Val: 100},
				{ID: 4, Val: 2097152},
				{ID: 9, Val: 1},
			},
			PseudoHeaderOrder: []string{
				":method",
				":scheme",
				":authority",
				":path",
			},
			Config: http.HTTP2Config{
				MaxReceiveBufferPerConnection: 10420225,
				MaxEncoderHeaderTableSize:     4096,
			},
		},
		Headers: http.Header{
			"Accept-Encoding": {acceptEncoding},
			"User-Agent": {fmt.Sprintf(
				"Mozilla/5.0 (%s) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%s %s",
				agent, version, tail,
			)},
		},
		HeaderOrder: []string{
			"host",
			"connection",
			"sec-fetch-dest",
			"user-agent",
			"accept",
			"sec-fetch-site",
			"sec-fetch-mode",
			"referer",
			"accept-language",
			"priority",
			"accept-encoding",
			"cookie",
		},
	}, nil
}
