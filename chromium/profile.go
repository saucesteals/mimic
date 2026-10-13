// Package chromium constructs Chromium-family browser profiles.
package chromium

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/saucesteals/mimic"
	"github.com/saucesteals/mimic/wire"
)

// Platform identifies the browser operating system.
type Platform string

const (
	Windows Platform = "win"
	Mac     Platform = "mac"
	Linux   Platform = "linux"
	Android Platform = "android"
)

// Brand identifies a Chromium-based browser.
type Brand string

const (
	Chrome Brand = "Google Chrome"
	Brave  Brand = "Brave"
	Edge   Brand = "Microsoft Edge"
)

// Config selects a Chromium profile. All fields are required.
type Config struct {
	Brand    Brand
	Version  string
	Platform Platform
}

// ErrUnsupportedVersion identifies versions without a supported profile.
var ErrUnsupportedVersion = errors.New("chromium: unsupported version")

func http2Profile(majorVersion int) *mimic.HTTP2Profile {
	const tableSize, streamWindow = 65536, 6291456
	settings := []mimic.HTTP2Setting{{ID: 1, Val: tableSize}, {ID: 2, Val: 0}}
	if majorVersion < 120 {
		settings = append(settings, mimic.HTTP2Setting{ID: 3, Val: 1000})
	}
	settings = append(settings,
		mimic.HTTP2Setting{ID: 4, Val: streamWindow},
		mimic.HTTP2Setting{ID: 6, Val: 262144},
	)

	return &mimic.HTTP2Profile{
		PseudoHeaderOrder: []string{
			":method",
			":authority",
			":scheme",
			":path",
		},
		Settings: settings,
		Config: http.HTTP2Config{
			MaxReceiveBufferPerConnection: 15663105,
			MaxEncoderHeaderTableSize:     tableSize,
		},
		Priority: &mimic.HTTP2Priority{
			Exclusive: true,
			Weight:    255,
		},
	}
}

// New returns a fresh profile for Chromium 107–154, excluding draft-Kyber versions.
// Brand selects the default headers; platform also selects TLS key shares.
func New(config Config) (*mimic.Profile, error) {
	brand, version, platform := config.Brand, config.Version, config.Platform
	majorVersion, _, _ := strings.Cut(version, ".")
	majorVersionNumber, err := strconv.Atoi(majorVersion)
	if err != nil {
		return nil, err
	}

	if majorVersionNumber < 107 || majorVersionNumber > 154 {
		return nil, ErrUnsupportedVersion
	}

	if majorVersionNumber >= 115 && majorVersionNumber < 120 || majorVersionNumber >= 124 && majorVersionNumber < 131 {
		return nil, fmt.Errorf(
			"%w: Chromium %d requires obsolete draft-Kyber not implemented by native Go",
			ErrUnsupportedVersion, majorVersionNumber,
		)
	}

	getSpec := func() (*wire.ClientHelloProfile, error) { return clientHello(majorVersionNumber, platform) }

	var agent, platformName string
	mobile, suffix := "?0", ""
	switch platform {
	case Windows:
		agent, platformName = "Windows NT 10.0; Win64; x64", "Windows"
	case Mac:
		agent, platformName = "Macintosh; Intel Mac OS X 10_15_7", "macOS"
	case Android:
		agent, platformName = "Linux; Android 10; K", "Android"
		mobile, suffix = "?1", " Mobile"
	case Linux:
		agent, platformName = "X11; Linux x86_64", "Linux"
	default:
		return nil, fmt.Errorf("chromium: unsupported platform: %s", platform)
	}

	if brand != Chrome && brand != Brave && brand != Edge {
		return nil, fmt.Errorf("chromium: unsupported brand: %s", brand)
	}
	userAgent := fmt.Sprintf(
		"Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s%s Safari/537.36",
		agent, version, suffix,
	)
	if brand == Edge {
		token := "Edg"
		if platform == Android {
			token = "EdgA"
		}
		userAgent += " " + token + "/" + version
	}
	return &mimic.Profile{
		ClientHello: getSpec,
		HTTP2:       http2Profile(majorVersionNumber),
		Headers: http.Header{
			"Sec-Ch-Ua":          {clientHintUA(brand, majorVersion, majorVersionNumber)},
			"Sec-Ch-Ua-Mobile":   {mobile},
			"Sec-Ch-Ua-Platform": {fmt.Sprintf("%q", platformName)},
			"User-Agent":         {userAgent},
		},
		HeaderOrder:         slices.Clone(chromeHeaderOrder),
		ResourceHeaderOrder: slices.Clone(chromeResourceHeaderOrder),
	}, nil
}

var chromeHeaderOrder = []string{
	"host",
	"connection",
	"sec-ch-ua",
	"sec-ch-ua-mobile",
	"sec-ch-ua-platform",
	"upgrade-insecure-requests",
	"user-agent",
	"accept",
	"sec-fetch-site",
	"sec-fetch-mode",
	"sec-fetch-user",
	"sec-fetch-dest",
	"referer",
	"accept-encoding",
	"accept-language",
	"cookie",
	"priority",
}

var chromeResourceHeaderOrder = []string{
	"host",
	"connection",
	"sec-ch-ua-platform",
	"user-agent",
	"sec-ch-ua",
	"sec-ch-ua-mobile",
	"accept",
	"sec-fetch-site",
	"sec-fetch-mode",
	"sec-fetch-dest",
	"referer",
	"accept-encoding",
	"accept-language",
	"cookie",
	"priority",
}
