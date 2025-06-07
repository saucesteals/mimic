# Mimic

[![GoDoc](https://godoc.org/github.com/saucesteals/mimic?status.svg)](https://godoc.org/github.com/saucesteals/mimic)

Mimic chromium's HTTP/HTTP2 and TLS implementations.

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
	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/mimic"
)

func main() {
	transport, _ := mimic.NewTransport(mimic.TransportOptions{
		Version:   "137.0.0.0",
		Brand:     mimic.BrandChrome,     // or mimic.BrandBrave, mimic.BrandEdge
		Platform:  mimic.PlatformWindows, // or mimic.PlatformMac, mimic.PlatformLinux
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	})

	client := &http.Client{Transport: transport}

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

	// req.Header[http.HeaderOrderKey] = []string{...} // optionally, you can set the order of the headers including the default ones from mimic

	res, _ := client.Do(req)

	body, _ := io.ReadAll(res.Body)
	fmt.Println(string(body))
}
```
