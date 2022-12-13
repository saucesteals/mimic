package mimic

import (
	"fmt"
	"strings"
)

type Brand string

var (
	legacyGreasyChars = []string{" ", " ", ";"}
	greasyChars       = []string{" ", "(", ":", "-", ".", "/", ")", ";", "=", "?", "_"}
	greasyVersion     = []string{"8", "99", "24"}
	greasyOrders      = [][]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2},
		{1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}

	BrandChrome Brand = "Google Chrome"
)

func formatBrand(brand Brand, majorVersion string) string {
	return fmt.Sprintf(`"%s";v="%s"`, brand, majorVersion)
}

func greasedBrand(majorVersionNumber int, seed int, permutedOrder []int) string {
	var brand, version string

	switch {
	case majorVersionNumber <= 102, majorVersionNumber == 104:
		brand = fmt.Sprintf("%sNot%sA%sBrand", legacyGreasyChars[permutedOrder[0]], legacyGreasyChars[permutedOrder[1]], legacyGreasyChars[permutedOrder[2]])
		version = "99"
	case majorVersionNumber == 103:
		brand = fmt.Sprintf("%sNot%sA%sBrand", greasyChars[(seed%(len(greasyChars)-1))+1], greasyChars[(seed+1)%len(greasyChars)], greasyChars[(seed+2)%len(greasyChars)])
		version = greasyVersion[seed%len(greasyVersion)]
	default: // >=105
		// https://github.com/WICG/ua-client-hints/pull/310
		brand = fmt.Sprintf("Not%sA%sBrand", greasyChars[seed%len(greasyChars)], greasyChars[(seed+1)%len(greasyChars)])
		version = greasyVersion[seed%len(greasyVersion)]
	}

	return formatBrand(Brand(brand), version)

}

func clientHintUA(brand Brand, majorVersion string, majorVersionNumber int) string {
	seed := majorVersionNumber
	if majorVersionNumber <= 102 {
		// legacy behavior (maybe a bug?)
		seed = 0
	}

	order := greasyOrders[seed%len(greasyOrders)]

	greased := make([]string, 3)

	greased[order[0]] = greasedBrand(majorVersionNumber, seed, order)
	greased[order[1]] = formatBrand("Chromium", majorVersion)
	greased[order[2]] = formatBrand(brand, majorVersion)

	return strings.Join(greased, ", ")
}
