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

func greasedBrand(majorVersion int, permutation []int) string {
	var brand, version string

	switch {
	case majorVersion <= 103:
		brand = fmt.Sprintf("%sNot%sA%sBrand", greasyChars[(majorVersion%(len(greasyChars)-1))+1], greasyChars[(majorVersion+1)%len(greasyChars)], greasyChars[(majorVersion+2)%len(greasyChars)])
		version = greasyVersion[majorVersion%len(greasyVersion)]
	case majorVersion == 104:
		// updated grease disabled for some reason?
		brand = fmt.Sprintf("%sNot%sA%sBrand", legacyGreasyChars[permutation[0]], legacyGreasyChars[permutation[1]], legacyGreasyChars[permutation[2]])
		version = "99"
	default: // >=105
		// https://github.com/WICG/ua-client-hints/pull/310
		brand = fmt.Sprintf("Not%sA%sBrand", greasyChars[majorVersion%len(greasyChars)], greasyChars[(majorVersion+1)%len(greasyChars)])
		version = greasyVersion[majorVersion%len(greasyVersion)]
	}

	return formatBrand(Brand(brand), version)

}

func clientHintUA(brand Brand, iMajorVersion int, majorVersion string, version string) string {
	order := greasyOrders[iMajorVersion%len(greasyOrders)]

	greased := make([]string, 3)

	greased[order[0]] = greasedBrand(iMajorVersion, order)
	greased[order[1]] = formatBrand("Chromium", majorVersion)
	greased[order[2]] = formatBrand(brand, majorVersion)

	return strings.Join(greased, ", ")
}
