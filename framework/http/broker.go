package http

import (
	"context"
	"encoding/json"
	httpstd "net/http"
	"sync/atomic"
	"time"
)

// Default broker configuration.
const (
	DefaultHeartbeatInterval = 15 * time.Second
	DefaultClientBuffer      = 16
)

// BrokerStats holds current broker connection and message statistics.
// All fields are safe to read concurrently from any goroutine.
type BrokerStats struct {
	Clients   int   `json:"clients"`
	Topics    int   `json:"topics"`
	Published int64 `json:"published"`
	Errors    int64 `json:"errors"`
}

// BrokerOption configures a [Broker].
type BrokerOption func(*Broker)

// WithHeartbeatInterval sets how often the broker sends keep-alive pings
// to all connected clients. Default is 15 seconds.
func WithHeartbeatInterval(d time.Duration) BrokerOption {
	return func(b *Broker) { b.heartbeat = d }
}

// WithClientBuffer sets the per-client message buffer size. When a client's
// buffer is full, new messages for that client are dropped and the error
// counter increments. Default is 16.
func WithClientBuffer(n int) BrokerOption {
	return func(b *Broker) { b.bufSize = n }
}

// Broker manages multiple SSE client connections with topic-based fan-out.
//
// Create a broker with [NewBroker], start its event loop with [Broker.Run]
// (typically in a goroutine), and call [Broker.Subscribe] from HTTP handlers
// to connect clients. Use [Broker.Publish] or [Broker.Broadcast] from any
// goroutine to push events.
//
//	broker := http.NewBroker()
//	go broker.Run(ctx)
//
//	// HTTP handler — blocks until client disconnects:
//	func handleEvents(w http.ResponseWriter, r *http.Request) {
//	    broker.Subscribe(w, r, "user:123", "global")
//	}
//
//	// From a background worker or HTTP handler:
//	broker.Publish("user:123", "notification", payload)
//
// The broker runs a single event-loop goroutine. All map operations happen
// on that goroutine, so no locks are needed for client/topic management.
// Per-client message delivery uses non-blocking sends — slow clients get
// dropped messages rather than blocking the entire fan-out.
type Broker struct {
	register   chan *brokerClient
	unregister chan *brokerClient
	messages   chan brokerMessage
	done       chan struct{}

	heartbeat time.Duration
	bufSize   int

	clientCount atomic.Int32
	topicCount  atomic.Int32
	published   atomic.Int64
	errors      atomic.Int64
}

type brokerClient struct {
	send   chan brokerMessage
	topics []string
}

type brokerMessage struct {
	event     string
	data      string
	id        string
	topic     string // routing key; empty = broadcast
	heartbeat bool   // true = keep-alive ping, not a real event
}

// NewBroker creates a new SSE broker. Call [Broker.Run] to start the
// event loop before subscribing clients or publishing events.
func NewBroker(opts ...BrokerOption) *Broker {
	b := &Broker{
		register:   make(chan *brokerClient, 16),
		unregister: make(chan *brokerClient, 16),
		messages:   make(chan brokerMessage, 256),
		done:       make(chan struct{}),
		heartbeat:  DefaultHeartbeatInterval,
		bufSize:    DefaultClientBuffer,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Run starts the broker's event loop. It blocks until ctx is cancelled,
// at which point all connected clients are disconnected gracefully.
// Typically called in a goroutine:
//
//	go broker.Run(ctx)
func (b *Broker) Run(ctx context.Context) {
	ticker := time.NewTicker(b.heartbeat)
	defer ticker.Stop()

	clients := make(map[*brokerClient]struct{})
	topics := make(map[string]map[*brokerClient]struct{})

	removeClient := func(c *brokerClient) {
		if _, ok := clients[c]; !ok {
			return
		}
		delete(clients, c)
		for _, t := range c.topics {
			delete(topics[t], c)
			if len(topics[t]) == 0 {
				delete(topics, t)
			}
		}
		close(c.send)
		b.clientCount.Store(int32(len(clients)))
		b.topicCount.Store(int32(len(topics)))
	}

	for {
		select {
		case <-ctx.Done():
			for c := range clients {
				close(c.send)
			}
			b.clientCount.Store(0)
			b.topicCount.Store(0)
			close(b.done)
			return

		case c := <-b.register:
			clients[c] = struct{}{}
			for _, t := range c.topics {
				if topics[t] == nil {
					topics[t] = make(map[*brokerClient]struct{})
				}
				topics[t][c] = struct{}{}
			}
			b.clientCount.Store(int32(len(clients)))
			b.topicCount.Store(int32(len(topics)))

		case c := <-b.unregister:
			removeClient(c)

		case msg := <-b.messages:
			var targets map[*brokerClient]struct{}
			if msg.topic == "" {
				targets = clients
			} else {
				targets = topics[msg.topic]
			}
			for c := range targets {
				select {
				case c.send <- msg:
				default:
					b.errors.Add(1)
				}
			}

		case <-ticker.C:
			hb := brokerMessage{heartbeat: true}
			for c := range clients {
				select {
				case c.send <- hb:
				default:
					// skip heartbeat for slow clients
				}
			}
		}
	}
}

// Subscribe upgrades the HTTP connection to SSE and subscribes to the given
// topics. It blocks until the client disconnects or the broker shuts down.
// Returns [ErrStreamingNotSupported] if the ResponseWriter cannot flush.
//
// A client with no topics receives only [Broker.Broadcast] messages.
// Topics are typically user IDs, channel names, or resource identifiers:
//
//	broker.Subscribe(w, r, "user:"+userID, "global")
func (b *Broker) Subscribe(w httpstd.ResponseWriter, r *httpstd.Request, topics ...string) error {
	stream, err := NewEventStream(w, r)
	if err != nil {
		return err
	}

	client := &brokerClient{
		send:   make(chan brokerMessage, b.bufSize),
		topics: topics,
	}

	// Register with the event loop.
	select {
	case b.register <- client:
	case <-b.done:
		return ErrBrokerStopped
	}

	// Unregister on exit. Uses b.done to avoid blocking if the broker
	// shut down while this client was connected.
	defer func() {
		select {
		case b.unregister <- client:
		case <-b.done:
		}
	}()

	// Write loop: drain per-client buffer, write to SSE stream.
	for {
		select {
		case <-stream.Done():
			return nil
		case <-b.done:
			return nil
		case msg, ok := <-client.send:
			if !ok {
				return nil
			}
			if msg.heartbeat {
				if err := stream.Heartbeat(); err != nil {
					return nil
				}
				continue
			}
			if err := stream.Send(msg.event, msg.data, msg.id); err != nil {
				return nil
			}
		}
	}
}

// Publish sends an event to all clients subscribed to the given topic.
// The data is JSON-encoded before delivery. If encoding fails, the error
// counter increments and the message is discarded.
//
// Blocks if the broker's internal message buffer is full (capacity 256).
// For fan-out to all clients regardless of topic, use [Broker.Broadcast].
func (b *Broker) Publish(topic, event string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		b.errors.Add(1)
		return
	}
	select {
	case b.messages <- brokerMessage{topic: topic, event: event, data: string(raw)}:
		b.published.Add(1)
	case <-b.done:
	}
}

// Broadcast sends an event to all connected clients regardless of topic
// subscription. Equivalent to [Broker.Publish] with an empty topic.
func (b *Broker) Broadcast(event string, data any) {
	b.Publish("", event, data)
}

// Stats returns current broker statistics. Safe for concurrent use from
// any goroutine.
func (b *Broker) Stats() BrokerStats {
	return BrokerStats{
		Clients:   int(b.clientCount.Load()),
		Topics:    int(b.topicCount.Load()),
		Published: b.published.Load(),
		Errors:    b.errors.Load(),
	}
}

// ErrBrokerStopped indicates the broker has been shut down and can no
// longer accept subscriptions.
var ErrBrokerStopped = NewError(
	httpstd.StatusServiceUnavailable,
	"broker_stopped",
	"Event broker is not running",
)
