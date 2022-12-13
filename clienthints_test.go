package mimic

import (
	"testing"
)

func TestClientHintUA(t *testing.T) {
	tests := []struct {
		version      string
		clientHintUa string
	}{
		{`80.0.0.0`, `" Not A;Brand";v="99", "Chromium";v="80", "Google Chrome";v="80"`},
		{"98.0.0.0", `" Not A;Brand";v="99", "Chromium";v="98", "Google Chrome";v="98"`},
		{"99.0.0.0", `" Not A;Brand";v="99", "Chromium";v="99", "Google Chrome";v="99"`},
		{"100.0.0.0", `" Not A;Brand";v="99", "Chromium";v="100", "Google Chrome";v="100"`},
		{"101.0.0.0", `" Not A;Brand";v="99", "Chromium";v="101", "Google Chrome";v="101"`},
		{"102.0.0.0", `" Not A;Brand";v="99", "Chromium";v="102", "Google Chrome";v="102"`},
		{`103.0.0.0`, `".Not/A)Brand";v="99", "Google Chrome";v="103", "Chromium";v="103"`},
		{`104.0.0.0`, `"Chromium";v="104", " Not A;Brand";v="99", "Google Chrome";v="104"`},
		{`105.0.0.0`, `"Google Chrome";v="105", "Not)A;Brand";v="8", "Chromium";v="105"`},
		{`106.0.0.0`, `"Chromium";v="106", "Google Chrome";v="106", "Not;A=Brand";v="99"`},
		{`107.0.0.0`, `"Google Chrome";v="107", "Chromium";v="107", "Not=A?Brand";v="24"`},
		{`108.0.0.0`, `"Not?A_Brand";v="8", "Chromium";v="108", "Google Chrome";v="108"`},
	}

	for _, test := range tests {
		majorVersion, majorVersionNumber, err := getMajor(test.version)
		if err != nil {
			t.Fatal(err)
		}

		ua := clientHintUA(BrandChrome, majorVersion, majorVersionNumber)

		if ua != test.clientHintUa {
			t.Errorf("want %s; got %s", test.clientHintUa, ua)
		}
	}

}
