package middleware

import (
	"strings"
	"testing"
)

func FuzzActionID(f *testing.F) {
	for _, seed := range []string{"order-1", "", "a b", "\r\n", strings.Repeat("a", 129)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		want := len(input) > 0 && len(input) <= 128
		for i := 0; i < len(input); i++ {
			if input[i] < 33 || input[i] > 126 {
				want = false
			}
		}
		if validActionID(input) != want {
			t.Fatalf("action ID validation differs for %q", input)
		}
	})
}
