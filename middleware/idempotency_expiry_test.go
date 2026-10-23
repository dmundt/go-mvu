package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIdempotencySchedulesSuccessiveExpirySweeps(t *testing.T) {
	options := phase4IdempotencyOptions()
	options.MaxEntries, options.TTL = 1, 10*time.Millisecond
	adapter, err := Idempotency(options)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	handler := adapter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	}))
	for cycle := 1; cycle <= 3; cycle++ {
		writer := httptest.NewRecorder()
		handler.ServeHTTP(writer, phase4ActionRequest("same", "{}"))
		if writer.Code != http.StatusCreated || calls != cycle {
			t.Fatalf("cycle %d: status=%d calls=%d", cycle, writer.Code, calls)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
