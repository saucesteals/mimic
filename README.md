# Mimic

[![Go Reference](https://pkg.go.dev/badge/github.com/saucesteals/mimic.svg)](https://pkg.go.dev/github.com/saucesteals/mimic)

Browser impersonation for Go's `net/http`. Chromium, Firefox and Safari profiles
for TLS, HTTP/2 and request headers, behind a standard `http.RoundTripper`.
Go's `http.Transport` still owns connection pooling, flow control, retries and
cancellation. Keep your `http.Client`, cookie jar, middleware and `httptrace`.

[API reference](https://pkg.go.dev/github.com/saucesteals/mimic) · [Usage](#usage) · [Configuration](#configuration)

## Install

Requires Go 1.27.2 or later.

```sh
go get github.com/saucesteals/mimic@v1.1.0-beta.2
```

This is a breaking redesign of v1.0.x. The module path is unchanged; migrate
from `fhttp` to `net/http` and from `TransportOptions` to the profile and
`mimic.Config` API shown below. Existing applications can stay on v1.0.2.

## Usage

Create a browser profile, then use its transport with `http.Client`:

```go
package main

import (
	"net/http"

	"github.com/saucesteals/mimic"
	"github.com/saucesteals/mimic/chromium"
)

func main() {
	// Firefox and Safari profiles are available via firefox.New and safari.New.
	profile, err := chromium.New(chromium.Config{
		Brand:    chromium.Chrome, // or chromium.Brave, chromium.Edge
		Version:  "154.0.0.0",
		Platform: chromium.Mac, // or chromium.Windows, chromium.Linux, chromium.Android
	})
	if err != nil {
		panic(err)
	}

	transport, err := mimic.NewTransport(mimic.Config{
		Profile: profile,
		HTTP: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	})
	if err != nil {
		panic(err)
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport}
	resp, err := client.Get("https://h2.tls.but.sh/api/network")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}
```

## Browser profiles

Each browser package provides `New(Config)`. Set the version and platform
explicitly, and the brand for Chromium.

- `chromium`: Chrome, Brave and Edge 107–154 on Windows, macOS, Linux and Android.
  Versions 115–119 and 124–130 require obsolete draft-Kyber and are rejected.
- `firefox`: Firefox 157 on Windows, macOS, Linux and Android.
- `safari`: Safari 26.0–26.3, including 26.0.1, on macOS, iPhone and iPad desktop mode.
  The initial 26.0 TLS snapshots differ by platform; later versions use the hybrid snapshot.

Profiles supply User-Agent and header defaults, including Chromium client hints.
For Chrome or Firefox on iOS, use Safari's wire profile and set the appropriate
request headers.

These are versioned wire profiles, not a browser runtime. A different version
string does not reproduce an arbitrary browser release. Request semantics and
response-body decoding remain the caller's responsibility.

## Configuration

`mimic.NewTransport` accepts a `mimic.Config` with three fields:

- `Profile`: required. Defines headers, header order, HTTP/2 settings and the
  `ClientHello` factory. Finish configuring it before use; do not mutate it afterward.
- `HTTP`: a `*http.Transport` to clone. Defaults to `http.DefaultTransport`.
- `TLS`: a `*tlsx.Config` for origin TLS, including root CAs, client certificates,
  verification callbacks and session caching. Nil uses defaults.

HTTP and TLS configurations are cloned, not modified. Reuse the transport across
requests and call `CloseIdleConnections` when retiring it. The profile controls
ALPN and TLS compatibility; custom TLS dialers are not supported.

`HTTP.TLSClientConfig` configures HTTPS proxies, not origin TLS. Proxies are chosen
at dial time, and pooled tunnels keep their original proxy. Use separate
transports when proxy isolation matters.

### Explicit header order

Use the profile's ordering by default, or override regular headers per request:

```go
req.Header[mimic.HeaderOrderKey] = []string{
	"user-agent", "accept", "x-custom",
}
```

Listed headers come first, then profile order, then lexical order. HTTP/2
pseudoheaders keep their profile order. The ordering key is local metadata,
removed before transmission. Its invalid header name makes stock `net/http`
reject it if the request accidentally bypasses Mimic.

## TLS implementation

The TLS dependency, [`tlsx`](https://github.com/saucesteals/tlsx), is maintained
as a single commit over Go's `crypto/tls`. Its browser-compatibility layer is
**280 lines of code changes**: 56 added or removed in upstream files and 224 in
`tlsx.go`. That count excludes imports, comments, blank lines and portability
helpers; it is not the size of the entire fork. Browser profiles and protocol
extensions live in Mimic, while Go's TLS implementation handles certificate
verification, key exchange and handshake transcripts.

## Contributing

All pull requests are welcome. I aim to keep Mimic as slim as possible: small,
focused changes, minimal dependencies, and standard-library behavior wherever
possible. For a larger change, open an issue first so we can agree on the approach.

## License

[MIT](LICENSE). The `tlsx` dependency retains Go's BSD license.
