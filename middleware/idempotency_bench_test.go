package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

func BenchmarkIdempotencySaturation(b *testing.B) {
	for _, size := range []int{128, 4096, 16384} {
		for _, mode := range []string{"replay", "reservation-full"} {
			b.Run(fmt.Sprintf("%s/entries=%d", mode, size), func(b *testing.B) {
				options := phase4IdempotencyOptions()
				options.MaxEntries = size
				options.TTL = time.Hour
				options.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
					var public *httpx.Error
					if !errors.As(err, &public) {
						b.Errorf("unexpected error: %v", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.WriteHeader(public.Status)
				}
				adapter, err := Idempotency(options)
				if err != nil {
					b.Fatal(err)
				}
				handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				keys := make([]string, size)
				for i := range keys {
					keys[i] = fmt.Sprint(i)
					writer := httptest.NewRecorder()
					handler.ServeHTTP(writer, phase4ActionRequest(keys[i], "{}"))
					if writer.Code != http.StatusOK {
						b.Fatalf("prefill status %d", writer.Code)
					}
				}
				var sequence atomic.Uint64
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						i := sequence.Add(1)
						key, want := keys[i%uint64(size)], http.StatusOK
						if mode == "reservation-full" {
							key, want = fmt.Sprintf("new-%d", i), http.StatusServiceUnavailable
						}
						writer := httptest.NewRecorder()
						handler.ServeHTTP(writer, phase4ActionRequest(key, "{}"))
						if writer.Code != want {
							b.Errorf("status %d, want %d", writer.Code, want)
						}
					}
				})
			})
		}
	}
}
