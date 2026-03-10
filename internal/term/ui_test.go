package term

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

// flushWriter is a threadsafe buffer that also implements Flush.
type flushWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *flushWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *flushWriter) Flush() error {
	return nil
}

func (w *flushWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *flushWriter) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Len()
}

// TestStartSpinner_StopIdempotent ensures that calling the stop function
// returned by StartSpinner multiple times does not panic and that no further
// writes occur after the first stop.
func TestStartSpinner_StopIdempotent(t *testing.T) {
	w := &flushWriter{}

	stop := StartSpinner(w, "testing spinner")

	// Allow a couple of frames to be written.
	time.Sleep(200 * time.Millisecond)

	// First stop: should complete without panic and clear the line.
	stop()
	afterFirstStopLen := w.Len()
	if afterFirstStopLen == 0 {
		t.Fatalf("expected some spinner output before stop, got none")
	}

	// Second stop: should be a no-op, not panic.
	stop()

	// Give time for any rogue writes; len should remain unchanged.
	time.Sleep(200 * time.Millisecond)
	afterSecondStopLen := w.Len()
	if afterSecondStopLen != afterFirstStopLen {
		t.Fatalf("expected no writes after stop, len changed from %d to %d", afterFirstStopLen, afterSecondStopLen)
	}

	// The buffer should end with the clear-line escape sequence.
	out := w.String()
	const clearSeq = "\r\033[K"
	if len(out) < len(clearSeq) || out[len(out)-len(clearSeq):] != clearSeq {
		t.Fatalf("expected output to end with %q, got %q", clearSeq, out)
	}
}

