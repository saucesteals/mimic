package chromium

import (
	"fmt"
	"strings"
)

var (
	greasyChars   = []string{" ", "(", ":", "-", ".", "/", ")", ";", "=", "?", "_"}
	greasyVersion = []string{"8", "99", "24"}
	greasyOrders  = [][]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2},
		{1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}
)

func formatBrand(brand Brand, majorVersion string) string {
	return fmt.Sprintf(`"%s";v="%s"`, brand, majorVersion)
}

func clientHintUA(brand Brand, majorVersion string, majorVersionNumber int) string {
	seed := majorVersionNumber
	order := greasyOrders[seed%len(greasyOrders)]

	greased := make([]string, 3)

	grease := fmt.Sprintf("Not%sA%sBrand", greasyChars[seed%len(greasyChars)], greasyChars[(seed+1)%len(greasyChars)])
	greased[order[0]] = formatBrand(Brand(grease), greasyVersion[seed%len(greasyVersion)])
	greased[order[1]] = formatBrand("Chromium", majorVersion)
	greased[order[2]] = formatBrand(brand, majorVersion)

	return strings.Join(greased, ", ")
}
