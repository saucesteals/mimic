package chrome

import (
	"testing"
)

func TestMimic(t *testing.T) {
	latest, err := GetLatestVersion(PlatformWindows)

	if err != nil {
		t.Fatal(err)
	}

	_, err = Mimic(latest)

	if err != nil {
		t.Fatal(err)
	}
}

func TestGrease(t *testing.T) {
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

		ua := clientHintUA(iMajor, major, test.version)

		if ua != test.clientHintUa {
			t.Errorf("want %s; got %s", test.clientHintUa, ua)
		}
	}

}
