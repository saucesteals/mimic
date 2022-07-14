package mimic

import (
	"testing"
)

func TestClientHintUA(t *testing.T) {
	tests := []struct {
		version      string
		clientHintUa string
	}{
		{`103.0.5060.114`, `".Not/A)Brand";v="99", "Google Chrome";v="103", "Chromium";v="103"`},
	}

	for _, test := range tests {

		major, iMajor, err := getMajor(test.version)

		if err != nil {
			t.Fatal(err)
		}

		ua := clientHintUA(iMajor, BrandChrome, major, test.version)

		if ua != test.clientHintUa {
			t.Errorf("want %s; got %s", test.clientHintUa, ua)
		}
	}

}
