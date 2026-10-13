package wire

import (
	"net/http"
	"slices"
	"strings"
)

// HeaderOrderKey is request-local metadata, not a valid HTTP header name.
// Stock net/http rejects requests containing it; Mimic consumes it before validation.
const HeaderOrderKey = "Mimic-Header-Order:"

const headerOrderKey = "Mimic-Header-Order"

// PrepareHeaderOrder moves metadata on Mimic's request copy to a private carrier.
// The HTTP/1 and HTTP/2 wire adapters remove the carrier before transmission.
func PrepareHeaderOrder(headers http.Header) {
	if order, ok := headers[HeaderOrderKey]; ok {
		headers[headerOrderKey] = order
		delete(headers, HeaderOrderKey)
	}
}

func headerOrder(value string) []string {
	var order []string
	for _, name := range strings.Split(value, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" && !strings.HasPrefix(name, ":") {
			order = append(order, name)
		}
	}

	return order
}

// compareHeaderNames puts configured headers first and orders unknown names lexically.
func compareHeaderNames(order []string, a, b string) int {
	rank := func(name string) int {
		if i := slices.Index(order, name); i >= 0 {
			return i
		}
		return len(order)
	}
	if d := rank(a) - rank(b); d != 0 {
		return d
	}

	return strings.Compare(a, b)
}
