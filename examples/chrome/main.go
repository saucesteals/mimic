package main

import (
	"encoding/json"
	"fmt"
	"os"

	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/mimic"
)

func main() {
	version := "137.0.0.0"
	if len(os.Args) > 1 {
		version = os.Args[1]
	}

	transport, err := mimic.NewTransport(mimic.TransportOptions{
		Version:   version,
		Brand:     mimic.BrandChrome,     // or mimic.BrandBrave, mimic.BrandEdge
		Platform:  mimic.PlatformWindows, // or mimic.PlatformMac, mimic.PlatformLinux
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	})
	if err != nil {
		panic(err)
	}

	client := &http.Client{Transport: transport}

	req, _ := http.NewRequest(http.MethodGet, "https://tls.peet.ws/api/clean", nil)

	// mimic automatically sets: user-agent, sec-ch-ua, sec-ch-ua-mobile, sec-ch-ua-platform
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

	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	defer res.Body.Close()

	var response PeetCleanResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s %s\n", req.Method, req.URL)
	for key, value := range req.Header {
		if key == http.HeaderOrderKey || key == http.PHeaderOrderKey {
			continue
		}
		fmt.Printf("  %s: %s\n", key, value[0])
	}
	fmt.Println()

	pprint("JA3", response.Ja3)
	pprint("JA3 Hash", response.Ja3Hash)
	pprint("JA4", response.Ja4)
	pprint("JA4-R", response.Ja4R)
	pprint("Akamai", response.Akamai)
	pprint("Akamai Hash", response.AkamaiHash)
	pprint("Peetprint", response.Peetprint)
	pprint("Peetprint Hash", response.PeetprintHash)
}

type PeetCleanResponse struct {
	Ja3           string `json:"ja3"`
	Ja3Hash       string `json:"ja3_hash"`
	Ja4           string `json:"ja4"`
	Ja4R          string `json:"ja4_r"`
	Akamai        string `json:"akamai"`
	AkamaiHash    string `json:"akamai_hash"`
	Peetprint     string `json:"peetprint"`
	PeetprintHash string `json:"peetprint_hash"`
}

func pprint(key string, value string) {
	fmt.Printf("%s:\n%s\n\n", key, value)
}
