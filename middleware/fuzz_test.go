package middleware

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func FuzzProxyTrust(f *testing.F) {
	for _, seed := range []string{"192.0.2.1", "198.51.100.1, 10.0.0.2", "", "::1", "bad, 10.0.0.2"} {
		f.Add(seed, "https")
	}
	f.Fuzz(func(t *testing.T, forwarded, proto string) {
		if len(forwarded)+len(proto) > 8192 {
			t.Skip()
		}
		wrap, err := TrustedProxies([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
		if err != nil {
			t.Fatal(err)
		}
		for _, peer := range []string{"203.0.113.1:1234", "10.0.0.1:1234"} {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = peer
			request.Header.Set("X-Forwarded-For", forwarded)
			request.Header.Set("X-Forwarded-Proto", proto)
			wrap(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				client, ok := ClientFromContext(r.Context())
				if !ok || (client.Scheme != "http" && client.Scheme != "https") {
					t.Fatalf("invalid client: %+v", client)
				}
				if strings.HasPrefix(peer, "203.") &&
					(client.Address.String() != "203.0.113.1" || client.Scheme != "http") {
					t.Fatalf("untrusted headers changed identity: %+v", client)
				}
				if peer == "10.0.0.1:1234" && (proto != "http" && proto != "https") && client.Scheme != "http" {
					t.Fatal("invalid forwarded scheme accepted")
				}
			})).ServeHTTP(httptest.NewRecorder(), request)
		}
	})
}

func FuzzTraceparent(f *testing.F) {
	for _, seed := range []string{
		"", "00-00000000000000000000000000000000-0000000000000000-01",
		"00-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
		"ff-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		trace, ok := parseTraceparent(input)
		if !ok {
			return
		}
		if len(input) != 55 || !strings.HasPrefix(input, "00-") ||
			len(trace.TraceID) != 32 || len(trace.ParentID) != 16 ||
			trace.TraceID == strings.Repeat("0", 32) || trace.ParentID == strings.Repeat("0", 16) ||
			strings.ToLower(input) != input {
			t.Fatalf("accepted malformed traceparent %q", input)
		}
		if _, err := hex.DecodeString(trace.TraceID + trace.ParentID); err != nil {
			t.Fatal(err)
		}
	})
}

func FuzzGzipNegotiation(f *testing.F) {
	for _, seed := range []string{"gzip", "gzip;q=0", "*;q=1", "gzip;q=NaN", "gzip;q=0,gzip;q=1", "gzip;q=1e0", "gzip;q=0.0001", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 8192 {
			t.Skip()
		}
		accepted := acceptsGzip(input)
		if accepted != acceptsGzip(input) {
			t.Fatal("negotiation is not deterministic")
		}
		if acceptsGzip(input + ",gzip;q=0") {
			t.Fatal("explicit zero quality did not disable gzip")
		}
		if input == strings.TrimSpace(input) && strings.HasPrefix(input, "gzip;q=") && !strings.Contains(input, ",") &&
			!strings.Contains(input[len("gzip;q="):], ";") {
			quality, valid := qualityValue(input[len("gzip;q="):])
			if accepted != (valid && quality > 0) {
				t.Fatal("malformed quality value enabled compression")
			}
		}
		if !strings.Contains(input, ",") && !strings.Contains(input, ";") &&
			!strings.EqualFold(strings.TrimSpace(input), "gzip") && strings.TrimSpace(input) != "*" && accepted {
			t.Fatalf("unknown coding accepted: %q", input)
		}

	})
}

func TestQualityValueSyntax(t *testing.T) {
	for _, value := range []string{"NaN", "+1", "-0", "1e0", ".5", "0.0001", "1.001", "2", "0..", "0;q=1", ""} {
		if _, valid := qualityValue(value); valid || acceptsGzip("gzip;q="+value) {
			t.Fatalf("accepted malformed qvalue %q", value)
		}
		if got := matchLanguage("de;q="+value, map[string]Locale{"de": "de"}); got != "" {
			t.Fatalf("malformed locale weight selected %q", got)
		}
	}
	for _, value := range []string{"0", "0.", "0.5", "0.001", "1", "1.", "1.000"} {
		if _, valid := qualityValue(value); !valid {
			t.Fatalf("rejected RFC qvalue %q", value)
		}
	}
}

func FuzzLocaleNegotiation(f *testing.F) {
	for _, seed := range []string{"en-US", "de;q=0.9,en;q=0.8", "", "*", "en;q=NaN", "fr;q=0"} {
		f.Add(seed)
	}
	supported := map[string]Locale{"en": "en", "de": "de", "fr": "fr"}
	f.Fuzz(func(t *testing.T, input string) {
		got := matchLanguage(input, supported)
		if got != "" && supported[string(got)] != got {
			t.Fatalf("unsupported locale selected: %q", got)
		}
		if got != matchLanguage(input, supported) {
			t.Fatal("locale selection is not deterministic")
		}
	})
}

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
