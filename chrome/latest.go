package chrome

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type platform string

var (
	errNoVersions = errors.New("chrome: no versions in history")

	PlatformWindows platform = "win"
)

type versionHistory struct {
	Versions []versions `json:"versions"`
}

type versions struct {
	Version string `json:"version"`
}

// GetLatestVersion returns the latest version of Chrome for the given platform.
func GetLatestVersion(pf platform) (string, error) {
	res, err := http.Get(fmt.Sprintf("https://versionhistory.googleapis.com/v1/chrome/platforms/%s/channels/stable/versions", pf))

	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	data := versionHistory{}

	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	if len(data.Versions) < 1 {
		return "", errNoVersions
	}

	return data.Versions[0].Version, nil
}

// MustGetLatestVersion is like GetLatestVersion but panics on error.
func MustGetLatestVersion(pf platform) string {
	version, err := GetLatestVersion(pf)

	if err != nil {
		panic(err)
	}

	return version
}
