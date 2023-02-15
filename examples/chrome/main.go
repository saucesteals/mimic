package main

import (
	"fmt"
	"io"
	"os"

	tls "github.com/refraction-networking/utls"
	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/mimic"
)

var (
	latestVersion = mimic.MustGetLatestVersion(mimic.PlatformWindows)
)

func main() {
	m, _ := mimic.Chromium(mimic.BrandChrome, latestVersion)

	ua := fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", m.Version())

	var keyLogFile *os.File
	keyLogPath := os.Getenv("SSLKEYLOGFILE")
	if keyLogPath != "" {
		var err error
		keyLogFile, err = os.OpenFile(keyLogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			panic(err)
		}
	}

	client := &http.Client{
		Transport: m.ConfigureTransport(&http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				KeyLogWriter: keyLogFile,
			},
		}),
	}

	req, _ := http.NewRequest("GET", "https://tls.peet.ws/api/all", nil)

	req.Header = http.Header{
		"sec-ch-ua":          {m.ClientHintUA()},
		"rtt":                {"50"},
		"sec-ch-ua-mobile":   {"?0"},
		"user-agent":         {ua},
		"accept":             {"text/html,*/*"},
		"x-requested-with":   {"XMLHttpRequest"},
		"downlink":           {"3.9"},
		"ect":                {"4g"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-site":     {"same-origin"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-dest":     {"empty"},
		"accept-encoding":    {"gzip, deflate, br"},
		"accept-language":    {"en,en_US;q=0.9"},
		http.HeaderOrderKey: {
			"sec-ch-ua", "rtt", "sec-ch-ua-mobile",
			"user-agent", "accept", "x-requested-with",
			"downlink", "ect", "sec-ch-ua-platform",
			"sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest",
			"accept-encoding", "accept-language",
		},
		http.PHeaderOrderKey: m.PseudoHeaderOrder(),
	}

	res, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		panic(err)
	}

	fmt.Println(string(body))
}
