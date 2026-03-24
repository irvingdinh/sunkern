package http

import (
	"encoding/json"
	httpstd "net/http"
	"strconv"
	"strings"
	"time"
)

// writeDeadlineExtension is how far into the future each SSE write extends
// the connection's write deadline. Must be longer than the heartbeat interval
// to prevent the server's WriteTimeout from killing idle streams.
const writeDeadlineExtension = 30 * time.Second

// SSEWriter writes Server-Sent Events to an HTTP response stream.
//
// Created by [NewEventStream]. Use [SSEWriter.Send] or [SSEWriter.SendJSON]
// to push events, [SSEWriter.Heartbeat] for keep-alive pings, and
// [SSEWriter.Done] to detect client disconnection.
//
// SSEWriter is not safe for concurrent use. The caller's event loop should
// be the single writer.
type SSEWriter struct {
	w    httpstd.ResponseWriter
	rc   *httpstd.ResponseController
	done <-chan struct{}
}

// NewEventStream prepares the response for Server-Sent Events streaming.
// It sets the required headers (Content-Type: text/event-stream, Cache-Control,
// Connection, X-Accel-Buffering) and flushes them to the client. Each
// subsequent write extends the write deadline to prevent server timeouts.
//
// The caller should loop, sending events and checking Done for disconnect:
//
//	stream, err := http.NewEventStream(w, r)
//	if err != nil {
//		http.Error(w, err)
//		return
//	}
//	ticker := time.NewTicker(15 * time.Second)
//	defer ticker.Stop()
//	for {
//		select {
//		case <-stream.Done():
//			return
//		case evt := <-events:
//			stream.SendJSON("update", evt, "")
//		case <-ticker.C:
//			stream.Heartbeat()
//		}
//	}
func NewEventStream(w httpstd.ResponseWriter, r *httpstd.Request) (*SSEWriter, error) {
	rc := httpstd.NewResponseController(w)

	// Extend the write deadline before committing headers. This overrides
	// the server's WriteTimeout for this connection, allowing the stream
	// to live beyond the default 15-second timeout.
	_ = rc.SetWriteDeadline(time.Now().Add(writeDeadlineExtension))

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(httpstd.StatusOK)

	if err := rc.Flush(); err != nil {
		return nil, ErrStreamingNotSupported
	}

	return &SSEWriter{
		w:    w,
		rc:   rc,
		done: r.Context().Done(),
	}, nil
}

// Send writes an SSE event to the stream.
//   - event: event type name (empty string omits the "event:" field)
//   - data: event payload (multi-line strings are split into separate "data:" lines)
//   - id: event ID for client reconnection via Last-Event-ID (empty string omits)
//
// Returns an error if the write or flush fails (typically client disconnect).
func (sw *SSEWriter) Send(event, data, id string) error {
	var b strings.Builder

	if id != "" {
		b.WriteString("id: ")
		b.WriteString(id)
		b.WriteByte('\n')
	}
	if event != "" {
		b.WriteString("event: ")
		b.WriteString(event)
		b.WriteByte('\n')
	}
	for _, line := range strings.Split(data, "\n") {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	return sw.write(b.String())
}

// SendJSON writes an SSE event with JSON-encoded data. Equivalent to
// calling [SSEWriter.Send] with json.Marshal'd data.
func (sw *SSEWriter) SendJSON(event string, v any, id string) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return sw.Send(event, string(data), id)
}

// Heartbeat sends an SSE comment as a keep-alive ping. Call this on a
// regular interval (e.g., every 15 seconds) to keep the connection alive
// through proxies and prevent server write timeouts.
func (sw *SSEWriter) Heartbeat() error {
	return sw.write(": heartbeat\n\n")
}

// Retry tells the client to wait ms milliseconds before reconnecting
// after a connection loss. Typically sent once after establishing the stream.
func (sw *SSEWriter) Retry(ms int) error {
	return sw.write("retry: " + strconv.Itoa(ms) + "\n\n")
}

// Done returns a channel that closes when the client disconnects.
func (sw *SSEWriter) Done() <-chan struct{} {
	return sw.done
}

// LastEventID extracts the Last-Event-ID header from the request. Clients
// send this when reconnecting after a dropped connection, allowing the
// server to replay missed events.
func LastEventID(r *httpstd.Request) string {
	return r.Header.Get("Last-Event-ID")
}

// write writes a string to the response, extends the write deadline, and
// flushes. Every SSE write goes through this to ensure timely delivery and
// to prevent the server's WriteTimeout from killing the connection.
func (sw *SSEWriter) write(s string) error {
	if _, err := sw.w.Write([]byte(s)); err != nil {
		return err
	}
	_ = sw.rc.SetWriteDeadline(time.Now().Add(writeDeadlineExtension))
	return sw.rc.Flush()
}

// ErrStreamingNotSupported indicates the ResponseWriter does not support
// flushing, which is required for Server-Sent Events. This typically means
// a middleware is wrapping the writer without propagating http.Flusher.
var ErrStreamingNotSupported = NewError(
	httpstd.StatusInternalServerError,
	"streaming_not_supported",
	"Streaming is not supported",
)
