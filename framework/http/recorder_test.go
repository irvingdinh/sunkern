package http

import (
	httpstd "net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseRecorderStatus(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.WriteHeader(httpstd.StatusCreated)

	if rec.Status() != 201 {
		t.Errorf("status = %d, want 201", rec.Status())
	}
}

func TestResponseRecorderImplicitOK(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.Write([]byte("hello"))

	if rec.Status() != 200 {
		t.Errorf("status = %d, want 200 (implicit on first Write)", rec.Status())
	}
}

func TestResponseRecorderBytesWritten(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.Write([]byte("hello"))
	rec.Write([]byte(" world"))

	if rec.BytesWritten() != 11 {
		t.Errorf("bytes = %d, want 11", rec.BytesWritten())
	}
}

func TestResponseRecorderDoubleWriteHeader(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.WriteHeader(httpstd.StatusNotFound)
	rec.WriteHeader(httpstd.StatusOK) // ignored

	if rec.Status() != 404 {
		t.Errorf("status = %d, want 404 (first WriteHeader wins)", rec.Status())
	}
}

func TestResponseRecorderFlush(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := NewResponseRecorder(underlying)

	// httptest.ResponseRecorder implements http.Flusher.
	rec.Flush()
	if !underlying.Flushed {
		t.Error("Flush was not delegated to underlying writer")
	}
}

func TestResponseRecorderUnwrap(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := NewResponseRecorder(underlying)

	if rec.Unwrap() != underlying {
		t.Error("Unwrap did not return the underlying writer")
	}
}

func TestResponseRecorderDefaultStatus(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())

	// Before any write, status should be 200.
	if rec.Status() != 200 {
		t.Errorf("default status = %d, want 200", rec.Status())
	}
	if rec.BytesWritten() != 0 {
		t.Errorf("default bytes = %d, want 0", rec.BytesWritten())
	}
}
