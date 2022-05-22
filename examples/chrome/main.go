package main

import (
	"fmt"
	"io"

	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/mimic"
)

func main() {

	client := &http.Client{
		Transport: mimic.Chrome101.NewTransport(nil),
		// Transport: mimic.Chrome101.NewTransport(
		// 	&http.Transport{
		// 		Proxy: ...,
		// 		...,
		// 	},
		// ),
	}

	req, _ := http.NewRequest("GET", "https://tls.peet.ws/api/all", nil)

	// Transport already injects default headers based on the spec (which you can override)
	// Just need to provide the correct header order
	req.Header = http.Header{
		http.HeaderOrderKey: {
			"cache-control", "upgrade-insecure-requests",
			"user-agent", "accept", "sec-gpc", "sec-fetch-site",
			"sec-fetch-mode", "sec-fetch-user", "sec-fetch-dest",
			"accept-encoding", "accept-language",
		},
		http.PHeaderOrderKey: mimic.Chrome101.PseudoHeaderOrder(),
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
