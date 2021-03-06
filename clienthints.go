package mimic

import (
	"fmt"
	"strings"
)

type Brand string

var (
	greasyChars   = []string{" ", "(", ":", "-", ".", "/", ")", ";", "=", "?", "_"}
	greasyVersion = []string{"8", "99", "24"}
	greasyOrders  = [][]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2},
		{1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}

	BrandChrome Brand = "Google Chrome"
)

func formatBrand(brand Brand, majorVersion string) string {
	return fmt.Sprintf(`"%s";v="%s"`, brand, majorVersion)
}

func greasedBrand(seed int) string {
	brand := fmt.Sprintf("%sNot%sA%sBrand", greasyChars[(seed%(len(greasyChars)-1))+1], greasyChars[(seed+1)%len(greasyChars)], greasyChars[(seed+2)%len(greasyChars)])
	version := greasyVersion[seed%len(greasyVersion)]
	return formatBrand(Brand(brand), version)
}

func clientHintUA(seed int, brand Brand, majorVersion string, version string) string {
	order := greasyOrders[seed%len(greasyOrders)]

	greased := make([]string, 3)

	greased[order[0]] = greasedBrand(seed)
	greased[order[1]] = formatBrand("Chromium", majorVersion)
	greased[order[2]] = formatBrand(brand, majorVersion)

	return strings.Join(greased, ", ")
}
