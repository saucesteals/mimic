// Package safari constructs Safari TLS and HTTP/2 profiles.
package safari

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

// New returns a Safari 26+ profile, including the 26.0.1 TLS update.
// Newer versions reuse the latest implemented wire behavior.
// HTTP/2 uses the Safari 26 settings shared by the supported platforms.
func New(config Config) (*mimic.Profile, error) {
	version, platform := config.Version, config.Platform
	parts := strings.Split(version, ".")
	if len(parts) > 3 {
		return nil, ErrUnsupportedVersion
	}
	var numbers [3]uint64
	for i, part := range parts {
		n, err := strconv.ParseUint(part, 10, 16)
		if err != nil || i == 0 && n < 26 {
			return nil, ErrUnsupportedVersion
		}
		numbers[i] = n
	}
	initial := numbers == [3]uint64{26, 0, 0}
	if len(parts) == 1 {
		version += ".0"
	}
	agent := "Macintosh; Intel Mac OS X 10_15_7"
	tail := "Safari/605.1.15"
	switch platform {
	case Mac, IPad:
	case IPhone:
		agent = "iPhone; CPU iPhone OS 18_7 like Mac OS X"
		if initial {
			agent = "iPhone; CPU iPhone OS 18_6 like Mac OS X"
		}
		tail = "Mobile/15E148 Safari/604.1"
	default:
		return nil, fmt.Errorf("safari: unsupported platform: %s", platform)
	}
	acceptEncoding := "gzip, deflate, br"
	if numbers[0] > 26 || numbers[1] >= 3 {
		// This snapshot assumes the matching OS networking stack.
		acceptEncoding += ", zstd"
	}
	return &mimic.Profile{
		ClientHello: func() (*wire.ClientHelloProfile, error) { return clientHello(initial, platform) },
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
