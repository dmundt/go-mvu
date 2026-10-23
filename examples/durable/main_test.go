package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/examples/durable/caskstore"
	"github.com/dmundt/go-mvu/store"
)

var csrfToken = regexp.MustCompile(`X-CSRF-Token&#34;:&#34;([^&]+)&#34;`)

func TestCounterEventIDSurvivesJSONForBinarySessionKeys(t *testing.T) {
	key := string([]byte{0xff, 0x80, 0x01})
	model, _, err := (CounterApp{}).Update(
		context.Background(), CounterModel{}, Increment{Key: key},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%s:1", base64.RawURLEncoding.EncodeToString([]byte(key)))
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var persisted CounterModel
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Outbox[0].ID != want {
		t.Fatalf("persisted event ID = %q, want %q", persisted.Outbox[0].ID, want)
	}
}

func TestCaskStorePersistsCompareAndSwapAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models")
	first, err := caskstore.Open[CounterModel](path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.Close() })
	created, err := first.Create(context.Background(), "session/key", CounterModel{Count: 2})
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 {
		t.Fatalf("create version = %d, want 1", created.Version)
	}
	if _, err := first.CompareAndSwap(context.Background(), "session/key", 0, CounterModel{}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale CAS error = %v, want conflict", err)
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := caskstore.Open[CounterModel](path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restarted.Close() })
	snapshot, err := restarted.Load(context.Background(), "session/key")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Model.Count != 2 || snapshot.Version != 1 {
		t.Fatalf("reloaded snapshot = %+v", snapshot)
	}
	snapshot, err = restarted.CompareAndSwap(
		context.Background(), "session/key", snapshot.Version, CounterModel{Count: 3},
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Model.Count != 3 || snapshot.Version != 2 {
		t.Fatalf("updated snapshot = %+v", snapshot)
	}
	keys, err := restarted.Keys()
	if err != nil || len(keys) != 1 || keys[0] != "session/key" {
		t.Fatalf("keys = %v, error = %v", keys, err)
	}
}

func TestOutboxDeliveryIsIdempotentAfterCrashBeforeAcknowledgement(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	models, err := caskstore.Open[CounterModel](filepath.Join(dataDir, "models"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { models.Close() })
	sink, err := NewFileSink(dataDir + "/delivered")
	if err != nil {
		t.Fatal(err)
	}
	event := OutboxEvent{ID: "session:1", Kind: "counter.incremented", Count: 1}
	if _, err := models.Create(ctx, "session", CounterModel{
		Count: 1, NextEventID: 1, Outbox: []OutboxEvent{event},
	}); err != nil {
		t.Fatal(err)
	}

	if err := sink.Deliver(ctx, event); err != nil {
		t.Fatal(err)
	}
	restartedModels, err := caskstore.Open[CounterModel](filepath.Join(dataDir, "models"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restartedModels.Close() })
	restartedSink, err := NewFileSink(dataDir + "/delivered")
	if err != nil {
		t.Fatal(err)
	}
	worker := outboxWorker{store: restartedModels, sink: restartedSink}
	if err := worker.Drain(ctx, "session"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := restartedModels.Load(ctx, "session")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Model.Outbox) != 0 {
		t.Fatalf("pending events = %v, want empty", snapshot.Model.Outbox)
	}
	deliveries, err := os.ReadDir(dataDir + "/delivered")
	if err != nil {
		t.Fatal(err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("delivery records = %d, want one idempotent delivery", len(deliveries))
	}
	data, err := os.ReadFile(dataDir + "/delivered/" + deliveries[0].Name())
	if err != nil {
		t.Fatal(err)
	}
	var delivered OutboxEvent
	if err := json.Unmarshal(data, &delivered); err != nil {
		t.Fatal(err)
	}
	if delivered.ID != event.ID {
		t.Fatalf("delivered event = %+v, want ID %q", delivered, event.ID)
	}
}

func TestDurableExamplePersistsSessionAndPendingOutbox(t *testing.T) {
	dir := t.TempDir()
	first, err := newService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.store.Close() })
	server := httptest.NewServer(first.handler)
	jar, err := cookiejar.New(nil)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	page := fetchPage(t, client, server.URL+"/")
	match := csrfToken.FindStringSubmatch(page)
	if match == nil {
		server.Close()
		t.Fatal("page missing CSRF token")
	}

	request, err := http.NewRequest(http.MethodPost, server.URL+"/increment", nil)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	request.Header.Set("HX-Request", "true")
	request.Header.Set("X-CSRF-Token", match[1])
	response, err := client.Do(request)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	server.Close()
	if err := first.store.Close(); err != nil {
		t.Fatal(err)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "Pending durable deliveries: 1") {
		t.Fatalf("increment response = %d %s", response.StatusCode, body)
	}

	keys, err := first.store.Keys()
	if err != nil || len(keys) != 1 {
		t.Fatalf("persisted session keys = %v, error = %v", keys, err)
	}
	snapshot, err := first.store.Load(context.Background(), keys[0])
	if err != nil {
		t.Fatal(err)
	}
	event := snapshot.Model.Outbox[0]
	if snapshot.Model.Count != 1 {
		t.Fatalf("committed count = %d, want 1", snapshot.Model.Count)
	}

	// Simulate delivery succeeding immediately before the process crashes,
	// leaving the durable event unacknowledged in the model store.
	if err := first.sink.Deliver(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	restarted, err := newService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restarted.store.Close() })
	restartedServer := httptest.NewServer(restarted.handler)
	defer restartedServer.Close()
	reloaded := fetchPage(t, client, restartedServer.URL+"/")
	if !strings.Contains(reloaded, "Pending durable deliveries: 1") ||
		!strings.Contains(reloaded, "<output>1</output>") {
		t.Fatalf("state did not survive restart: %s", reloaded)
	}
	if err := restarted.worker.Drain(context.Background(), keys[0]); err != nil {
		t.Fatal(err)
	}
	persisted, err := restarted.store.Load(context.Background(), keys[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Model.Outbox) != 0 {
		t.Fatalf("pending outbox after drain = %v", persisted.Model.Outbox)
	}
	deliveries, err := os.ReadDir(dir + "/delivered")
	if err != nil || len(deliveries) != 1 {
		t.Fatalf("durable idempotent deliveries = %d, error = %v", len(deliveries), err)
	}
}

func fetchPage(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", url, response.StatusCode, body)
	}
	return string(body)
}
