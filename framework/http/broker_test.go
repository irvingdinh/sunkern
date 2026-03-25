package http

import (
	"bufio"
	"context"
	httpstd "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// readSSEEvent reads one SSE event from the scanner. Events are terminated
// by a blank line. Comments (lines starting with ":") are captured as the
// "comment" field.
func readSSEEvent(scanner *bufio.Scanner) map[string]string {
	fields := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(fields) > 0 {
				break
			}
			continue // skip leading blank lines
		}
		if strings.HasPrefix(line, ":") {
			fields["comment"] = strings.TrimPrefix(line, ": ")
			continue
		}
		if i := strings.Index(line, ": "); i >= 0 {
			fields[line[:i]] = line[i+2:]
		}
	}
	return fields
}

// waitBrokerClients polls until the broker has exactly n clients.
func waitBrokerClients(t *testing.T, b *Broker, n int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for b.Stats().Clients != n {
		select {
		case <-deadline:
			t.Fatalf("timeout: wanted %d clients, got %d", n, b.Stats().Clients)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// connectSSE opens an SSE connection and returns the response and a scanner.
// The response body is closed automatically via t.Cleanup.
func connectSSE(t *testing.T, url string) (*httpstd.Response, *bufio.Scanner) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	req, _ := httpstd.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := httpstd.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp, bufio.NewScanner(resp.Body)
}

// readEventTimeout reads an SSE event with a deadline. Fails the test if
// no event arrives within the timeout.
func readEventTimeout(t *testing.T, scanner *bufio.Scanner, timeout time.Duration) map[string]string {
	t.Helper()
	ch := make(chan map[string]string, 1)
	go func() {
		ch <- readSSEEvent(scanner)
	}()
	select {
	case event := <-ch:
		return event
	case <-time.After(timeout):
		t.Fatal("timeout reading SSE event")
		return nil
	}
}

// newTestBroker creates a broker, starts it, and returns it with a stop
// function. The caller MUST call stop before closing the test server to
// ensure SSE handlers return before the server shuts down.
func newTestBroker(t *testing.T, opts ...BrokerOption) (*Broker, func()) {
	t.Helper()
	defaults := []BrokerOption{WithHeartbeatInterval(time.Hour)}
	b := NewBroker(append(defaults, opts...)...)
	ctx, cancel := context.WithCancel(context.Background())
	go b.Run(ctx)
	return b, cancel
}

// newTestServer creates an httptest.Server and registers cleanup that
// stops the broker first (so SSE handlers return), then closes the server.
func newTestServer(t *testing.T, stop func(), handler httpstd.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		stop()      // cancel broker → Subscribe returns → handlers finish
		srv.Close() // now safe to close
	})
	return srv
}

// ---------------------------------------------------------------------------
// Broker
// ---------------------------------------------------------------------------

func TestBrokerPublishSubscribe(t *testing.T) {
	broker, stop := newTestBroker(t)
	srv := newTestServer(t, stop, httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
		broker.Subscribe(w, r, "chat")
	}))

	_, scanner := connectSSE(t, srv.URL)
	waitBrokerClients(t, broker, 1)

	broker.Publish("chat", "message", map[string]string{"text": "hello"})

	event := readEventTimeout(t, scanner, 2*time.Second)
	if event["event"] != "message" {
		t.Errorf("event = %q, want %q", event["event"], "message")
	}
	if !strings.Contains(event["data"], "hello") {
		t.Errorf("data = %q, want to contain 'hello'", event["data"])
	}

	stats := broker.Stats()
	if stats.Clients != 1 {
		t.Errorf("clients = %d, want 1", stats.Clients)
	}
	if stats.Published != 1 {
		t.Errorf("published = %d, want 1", stats.Published)
	}
}

func TestBrokerBroadcast(t *testing.T) {
	broker, stop := newTestBroker(t)
	srv := newTestServer(t, stop, httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
		topic := r.URL.Query().Get("topic")
		broker.Subscribe(w, r, topic)
	}))

	_, scanner1 := connectSSE(t, srv.URL+"?topic=a")
	_, scanner2 := connectSSE(t, srv.URL+"?topic=b")
	waitBrokerClients(t, broker, 2)

	broker.Broadcast("ping", "pong")

	event1 := readEventTimeout(t, scanner1, 2*time.Second)
	event2 := readEventTimeout(t, scanner2, 2*time.Second)

	if event1["event"] != "ping" {
		t.Errorf("client1 event = %q, want %q", event1["event"], "ping")
	}
	if event2["event"] != "ping" {
		t.Errorf("client2 event = %q, want %q", event2["event"], "ping")
	}
}

func TestBrokerTopicIsolation(t *testing.T) {
	broker, stop := newTestBroker(t)
	srv := newTestServer(t, stop, httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
		topic := r.URL.Query().Get("topic")
		broker.Subscribe(w, r, topic)
	}))

	_, scannerA := connectSSE(t, srv.URL+"?topic=a")
	_, scannerB := connectSSE(t, srv.URL+"?topic=b")
	waitBrokerClients(t, broker, 2)

	// Publish to topic "a" only
	broker.Publish("a", "only-a", "for-a")

	eventA := readEventTimeout(t, scannerA, 2*time.Second)
	if eventA["event"] != "only-a" {
		t.Errorf("client A event = %q, want %q", eventA["event"], "only-a")
	}

	// Verify B didn't get it by sending a broadcast (B gets this, not "only-a")
	broker.Broadcast("check", "alive")

	eventB := readEventTimeout(t, scannerB, 2*time.Second)
	if eventB["event"] != "check" {
		t.Errorf("client B first event = %q, want broadcast 'check' (not 'only-a')", eventB["event"])
	}
}

func TestBrokerHeartbeat(t *testing.T) {
	broker, stop := newTestBroker(t, WithHeartbeatInterval(100*time.Millisecond))
	srv := newTestServer(t, stop, httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
		broker.Subscribe(w, r, "hb")
	}))

	_, scanner := connectSSE(t, srv.URL)
	waitBrokerClients(t, broker, 1)

	event := readEventTimeout(t, scanner, 2*time.Second)
	if event["comment"] != "heartbeat" {
		t.Errorf("expected heartbeat comment, got %v", event)
	}
}

func TestBrokerStatsMultipleTopics(t *testing.T) {
	broker, stop := newTestBroker(t)

	stats := broker.Stats()
	if stats.Clients != 0 || stats.Topics != 0 || stats.Published != 0 || stats.Errors != 0 {
		t.Errorf("initial stats not zero: %+v", stats)
	}

	srv := newTestServer(t, stop, httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
		broker.Subscribe(w, r, "x", "y")
	}))

	connectSSE(t, srv.URL)
	waitBrokerClients(t, broker, 1)

	stats = broker.Stats()
	if stats.Clients != 1 {
		t.Errorf("clients = %d, want 1", stats.Clients)
	}
	if stats.Topics != 2 {
		t.Errorf("topics = %d, want 2", stats.Topics)
	}
}

func TestBrokerClientDisconnect(t *testing.T) {
	// Short heartbeat ensures the server detects disconnect quickly
	// even if context cancellation is delayed.
	broker, stop := newTestBroker(t, WithHeartbeatInterval(50*time.Millisecond))
	srv := newTestServer(t, stop, httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
		broker.Subscribe(w, r, "dc")
	}))

	resp, _ := connectSSE(t, srv.URL)
	waitBrokerClients(t, broker, 1)

	resp.Body.Close()
	waitBrokerClients(t, broker, 0)

	if broker.Stats().Topics != 0 {
		t.Errorf("topics = %d, want 0 after disconnect", broker.Stats().Topics)
	}
}

func TestBrokerMultipleMessages(t *testing.T) {
	broker, stop := newTestBroker(t)
	srv := newTestServer(t, stop, httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
		broker.Subscribe(w, r, "seq")
	}))

	_, scanner := connectSSE(t, srv.URL)
	waitBrokerClients(t, broker, 1)

	for i := 0; i < 5; i++ {
		broker.Publish("seq", "tick", i)
	}

	for i := 0; i < 5; i++ {
		event := readEventTimeout(t, scanner, 2*time.Second)
		if event["event"] != "tick" {
			t.Errorf("event[%d] = %q, want %q", i, event["event"], "tick")
		}
	}

	if broker.Stats().Published != 5 {
		t.Errorf("published = %d, want 5", broker.Stats().Published)
	}
}

func TestBrokerNoTopicsOnlyBroadcast(t *testing.T) {
	broker, stop := newTestBroker(t)
	srv := newTestServer(t, stop, httpstd.HandlerFunc(func(w httpstd.ResponseWriter, r *httpstd.Request) {
		broker.Subscribe(w, r) // no topics
	}))

	_, scanner := connectSSE(t, srv.URL)
	waitBrokerClients(t, broker, 1)

	// Topic publish should NOT reach this client
	broker.Publish("private", "secret", "data")

	// Broadcast should reach it
	broker.Broadcast("global", "hello")

	event := readEventTimeout(t, scanner, 2*time.Second)
	if event["event"] != "global" {
		t.Errorf("event = %q, want %q (no-topic client should only get broadcasts)", event["event"], "global")
	}
}
