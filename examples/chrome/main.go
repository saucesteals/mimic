package main

import (
	"fmt"
	"io"

	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/mimic/chrome"
)

var (
	latestChrome = chrome.MustGetLatestVersion(chrome.PlatformWindows)
)

func main() {
	m, _ := chrome.Mimic(latestChrome)

	ua := fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", m.Version())

	client := &http.Client{Transport: m.ConfigureTransport(&http.Transport{ /* Proxy: ... */ })}

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
