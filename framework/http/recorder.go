package http

import httpstd "net/http"

// ResponseRecorder wraps an [http.ResponseWriter] to capture the status code
// and bytes written. Use it in custom middleware to inspect responses after
// the handler executes.
//
//	rec := http.NewResponseRecorder(w)
//	next.ServeHTTP(rec, r)
//	log.Printf("status=%d bytes=%d", rec.Status(), rec.BytesWritten())
//
// ResponseRecorder implements [http.Flusher] (delegating to the underlying
// writer) and Unwrap for [http.ResponseController] compatibility.
type ResponseRecorder struct {
	httpstd.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

// NewResponseRecorder wraps w and captures response metadata. The initial
// status is 200 (implicit HTTP success), matching net/http behavior where
// the first Write implicitly sends 200 if WriteHeader was not called.
func NewResponseRecorder(w httpstd.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{ResponseWriter: w, status: httpstd.StatusOK}
}

// WriteHeader captures the status code and delegates to the underlying
// writer. Only the first call takes effect; subsequent calls are ignored,
// matching net/http behavior.
func (r *ResponseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written and delegates to the underlying
// writer. Implicitly calls WriteHeader(200) on the first write.
func (r *ResponseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(httpstd.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += n
	return n, err
}

// Flush delegates to the underlying writer's Flush method if available.
func (r *ResponseRecorder) Flush() {
	if fl, ok := r.ResponseWriter.(httpstd.Flusher); ok {
		fl.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter. Required for
// [http.ResponseController] compatibility (Go 1.20+).
func (r *ResponseRecorder) Unwrap() httpstd.ResponseWriter {
	return r.ResponseWriter
}

// Status returns the HTTP status code written to the response.
func (r *ResponseRecorder) Status() int {
	return r.status
}

// BytesWritten returns the total number of bytes written to the response body.
func (r *ResponseRecorder) BytesWritten() int {
	return r.bytesWritten
}
