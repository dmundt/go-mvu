package middleware

import (
	"net/http"
	"strings"

	"github.com/dmundt/go-mvu/httpx"
)

// Extension is an optional, explicitly configured HTTP middleware.
type Extension func(http.Handler) http.Handler

func extensionError(handler httpx.ErrorHandler) httpx.ErrorHandler {
	if handler == nil {
		return httpx.RespondError
	}
	return handler
}

func rejectExtension(handler httpx.ErrorHandler, w http.ResponseWriter, r *http.Request, status int) {
	handler(w, r, &httpx.Error{Status: status})
}

// qualityValue accepts RFC 9110 qvalue syntax, not arbitrary floating-point
// literals such as NaN, exponents, signed values, or extra precision.
func qualityValue(value string) (float64, bool) {
	whole, fraction, dotted := strings.Cut(value, ".")
	if (whole != "0" && whole != "1") || (dotted && len(fraction) > 3) {
		return 0, false
	}
	result, scale := 0, 100
	for _, digit := range fraction {
		if digit < '0' || digit > '9' || (whole == "1" && digit != '0') {
			return 0, false
		}
		result += int(digit-'0') * scale
		scale /= 10
	}
	if whole == "1" {
		return 1, true
	}
	return float64(result) / 1000, true
}
