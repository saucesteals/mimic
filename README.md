# Mimic

[![GoDoc](https://godoc.org/github.com/saucesteals/mimic?status.svg)](https://godoc.org/github.com/saucesteals/mimic)

Mimic chromium's HTTP/HTTP2 and TLS implementations.

Requires Go 1.26 or later.

## TLS profiles

Chromium profiles include post-quantum key exchange for Chrome 124–130,
ML-DSA signature advertisement for Chrome 150+, and signature GREASE and
trust-anchor IDs for Chrome 152+. Supplemental TLS changes are isolated from
the base profiles until uTLS provides them.

ML-DSA advertisement does not enable ML-DSA certificate or handshake signature
verification. Trust-anchor IDs are profile data, not a replacement for the
configured certificate roots. Browser feature flags and root-store updates may
change the fingerprint; customize `ClientSpec.GetTlsSpec` when needed.

### Documentation

- [API Reference](https://godoc.org/github.com/saucesteals/mimic)
- [Example](https://github.com/saucesteals/mimic/blob/main/examples/chrome/main.go)

### Installation

```sh
go get github.com/saucesteals/mimic
```

## Usage

```go
package main

import (
	"fmt"
	"io"
	"time"

	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/mimic"
)

func main() {
	transport, _ := mimic.NewTransport(mimic.TransportOptions{
		Version:   "154.0.0.0",
		Brand:     mimic.BrandChrome,     // or mimic.BrandBrave, mimic.BrandEdge
		Platform:  mimic.PlatformWindows, // or mimic.PlatformMac, mimic.PlatformLinux
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	})

	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	req, _ := http.NewRequest(http.MethodGet, "https://tls.peet.ws/api/clean", nil)

	req.Header.Add("rtt", "50")
	req.Header.Add("accept", "text/html,*/*")
	req.Header.Add("x-requested-with", "XMLHttpRequest")
	req.Header.Add("downlink", "3.9")
	req.Header.Add("ect", "4g")
	req.Header.Add("sec-fetch-site", "same-origin")
	req.Header.Add("sec-fetch-mode", "cors")
	req.Header.Add("sec-fetch-dest", "empty")
	req.Header.Add("accept-encoding", "gzip, deflate, br")
	req.Header.Add("accept-language", "en,en_US;q=0.9")
	// mimic automatically sets: user-agent, sec-ch-ua, sec-ch-ua-mobile, sec-ch-ua-platform

	// optional header order
	// req.Header[http.HeaderOrderKey] = []string{
	// 	"user-agent", "sec-ch-ua", "sec-ch-ua-mobile", ...
	// }

	res, _ := client.Do(req)
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	fmt.Println(string(body))
}
```
