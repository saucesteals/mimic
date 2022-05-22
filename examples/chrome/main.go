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
	}

	req, _ := http.NewRequest("GET", "https://tls.peet.ws/api/all", nil)

	req.Header = http.Header{
		http.HeaderOrderKey: {
			"cache-control", "upgrade-insecure-requests",
			"user-agent", "accept", "sec-gpc", "sec-fetch-site",
			"sec-fetch-mode", "sec-fetch-user", "sec-fetch-dest",
			"accept-encoding", "accept-language",
		},
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
