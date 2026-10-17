// Package firefox constructs current Firefox desktop and Android profiles.
package firefox

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/saucesteals/mimic"
)

// Platform selects Firefox's default User-Agent.
type Platform string

const (
	Windows Platform = "win"
	Mac     Platform = "mac"
	Linux   Platform = "linux"
	Android Platform = "android"
)

// Config selects a Firefox profile. All fields are required.
type Config struct {
	Version  string
	Platform Platform
}

// ErrUnsupportedVersion identifies versions without a supported snapshot.
var ErrUnsupportedVersion = errors.New("firefox: unsupported version")

// New returns a Firefox 157 profile using normal (ticket-enabled) browsing.
// The snapshot follows Firefox 157 defaults, not uTLS's private/no-ticket 148
// preset. Header defaults describe navigation; callers own request semantics.
func New(config Config) (*mimic.Profile, error) {
	version, platform := config.Version, config.Platform
	parts := strings.Split(version, ".")
	if len(parts) > 3 || parts[0] != "157" {
		return nil, ErrUnsupportedVersion
	}
	for _, part := range parts {
		if _, err := strconv.ParseUint(part, 10, 16); err != nil {
			return nil, ErrUnsupportedVersion
		}
	}
	var agent string
	switch platform {
	case Windows:
		agent = "Windows NT 10.0; Win64; x64"
	case Mac:
		agent = "Macintosh; Intel Mac OS X 10.15"
	case Linux:
		agent = "X11; Linux x86_64"
	case Android:
		agent = "Android 10; Mobile"
	default:
		return nil, fmt.Errorf("firefox: unsupported platform: %s", platform)
	}
	gecko := "20100101"
	if platform == Android {
		gecko = "157.0"
	}
	return &mimic.Profile{
		ClientHello: clientHello,
		HTTP2: &mimic.HTTP2Profile{
			Settings: []mimic.HTTP2Setting{
				{ID: 1, Val: 65536},
				{ID: 2, Val: 0},
				{ID: 4, Val: 131072},
				{ID: 5, Val: 16384},
			},
			PseudoHeaderOrder: []string{
				":method",
				":path",
				":authority",
				":scheme",
			},
			Config: http.HTTP2Config{
				MaxReceiveBufferPerConnection: 12517377,
				MaxEncoderHeaderTableSize:     65536,
			},
			StreamReceiveWindow: 12582912,
			Priority:            &mimic.HTTP2Priority{Weight: 41},
		},
		Headers: http.Header{
			"Accept-Encoding": {"gzip, deflate, br, zstd"},
			"User-Agent": {fmt.Sprintf(
				"Mozilla/5.0 (%s; rv:157.0) Gecko/%s Firefox/157.0",
				agent, gecko,
			)},
		},
		HeaderOrder: []string{
			"host",
			"user-agent",
			"accept",
			"accept-language",
			"accept-encoding",
			"referer",
			"connection",
			"cookie",
			"upgrade-insecure-requests",
			"sec-fetch-dest",
			"sec-fetch-mode",
			"sec-fetch-site",
			"sec-fetch-user",
			"priority",
		},
	}, nil
}
