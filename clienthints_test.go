package mimic

import (
	"testing"
)

func TestClientHintUA(t *testing.T) {
	tests := []struct {
		version      string
		clientHintUa string
	}{
		{`103.0.0.0`, `".Not/A)Brand";v="99", "Google Chrome";v="103", "Chromium";v="103"`},
		{`104.0.0.0`, `"Chromium";v="104", " Not A;Brand";v="99", "Google Chrome";v="104"`},
		{`105.0.0.0`, `"Google Chrome";v="105", "Not)A;Brand";v="8", "Chromium";v="105"`},
		{`106.0.0.0`, `"Chromium";v="106", "Google Chrome";v="106", "Not;A=Brand";v="99"`},
		{`107.0.0.0`, `"Google Chrome";v="107", "Chromium";v="107", "Not=A?Brand";v="24"`},
		{`108.0.0.0`, `"Not?A_Brand";v="8", "Chromium";v="108", "Google Chrome";v="108"`},
	}

	for _, test := range tests {
		major, iMajor, err := getMajor(test.version)

		if err != nil {
			t.Fatal(err)
		}

		ua := clientHintUA(BrandChrome, iMajor, major, test.version)

		if ua != test.clientHintUa {
			t.Errorf("want %s; got %s", test.clientHintUa, ua)
		}
	}

}
