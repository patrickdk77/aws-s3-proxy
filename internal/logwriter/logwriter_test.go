package logwriter

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuf is an io.Writer safe for the run goroutine to write to while
// the test reads it.
type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// stalledWriter blocks every Write until release is closed, standing in
// for a container log pipe whose collector has stopped draining.
type stalledWriter struct {
	release chan struct{}
	writes  chan []byte
}

func newStalledWriter() *stalledWriter {
	return &stalledWriter{
		release: make(chan struct{}),
		writes:  make(chan []byte, 64),
	}
}

func (s *stalledWriter) Write(p []byte) (int, error) {
	<-s.release
	rec := make([]byte, len(p))
	copy(rec, p)
	select {
	case s.writes <- rec:
	default:
	}
	return len(p), nil
}

func TestWriteDeliversRecords(t *testing.T) {
	var out syncBuf
	lw := New(&out, 0)
	for _, s := range []string{"one\n", "two\n", "three\n"} {
		if _, err := lw.Write([]byte(s)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got := out.String()
	for _, want := range []string{"one", "two", "three"} {
		if !strings.Contains(got, want) {
			t.Errorf("output %q missing %q", got, want)
		}
	}
}

// TestWriteDoesNotBlockOnStalledSink is the regression test for the
// bug this package exists to prevent: a caller must return immediately
// even when the underlying pipe has stopped accepting writes entirely.
func TestWriteDoesNotBlockOnStalledSink(t *testing.T) {
	sink := newStalledWriter()
	lw := New(sink, 4)
	defer func() { close(sink.release); _ = lw.Close() }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Far more records than the queue holds, so the excess
		// must be dropped rather than parked on the sink.
		for i := 0; i < 1000; i++ {
			_, _ = lw.Write([]byte("line\n"))
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Write blocked while the sink was stalled")
	}
	if lw.Dropped() == 0 {
		t.Error("expected records to be dropped, got none")
	}
}

func TestDroppedRecordsAreReported(t *testing.T) {
	sink := newStalledWriter()
	lw := New(sink, 2)
	for i := 0; i < 200; i++ {
		_, _ = lw.Write([]byte("x\n"))
	}
	if lw.Dropped() == 0 {
		t.Fatal("expected drops with a stalled sink")
	}
	// Let the sink drain; the notice rides along with the next
	// batch so the loss is visible in the log itself.
	close(sink.release)
	_ = lw.Close()

	var all []byte
	for {
		select {
		case rec := <-sink.writes:
			all = append(all, rec...)
			continue
		default:
		}
		break
	}
	if !strings.Contains(string(all), "dropped") {
		t.Errorf("no drop notice in output: %q", string(all))
	}
}

func TestCloseFlushesQueuedRecords(t *testing.T) {
	var out syncBuf
	lw := New(&out, 0)
	for i := 0; i < 50; i++ {
		_, _ = lw.Write([]byte("queued\n"))
	}
	_ = lw.Close()
	if n := strings.Count(out.String(), "queued"); n != 50 {
		t.Errorf("flushed %d records, want 50", n)
	}
}

func TestWriteAfterCloseDoesNotPanic(t *testing.T) {
	var out syncBuf
	lw := New(&out, 0)
	_ = lw.Close()
	// Handlers can still be in flight during shutdown; these must
	// be discarded rather than panicking on a closed channel.
	for i := 0; i < 10; i++ {
		if _, err := lw.Write([]byte("late\n")); err != nil {
			t.Fatalf("Write after Close: %v", err)
		}
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	var out syncBuf
	lw := New(&out, 0)
	_ = lw.Close()
	_ = lw.Close()
	_ = lw.Close()
}

// TestWriteCopiesCallerBuffer guards the io.Writer contract: fmt and
// slog both reuse their formatting buffer after Write returns, so
// retaining p rather than copying it would corrupt the output.
func TestWriteCopiesCallerBuffer(t *testing.T) {
	var out syncBuf
	lw := New(&out, 0)
	p := []byte("first\n")
	_, _ = lw.Write(p)
	copy(p, []byte("XXXXX\n"))
	_ = lw.Close()
	if got := out.String(); !strings.Contains(got, "first") {
		t.Errorf("record was not copied, got %q", got)
	}
}

func TestConcurrentWritesAreSafe(t *testing.T) {
	var out syncBuf
	lw := New(&out, 0)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = lw.Write([]byte("concurrent\n"))
			}
		}()
	}
	wg.Wait()
	_ = lw.Close()
	if n := strings.Count(out.String(), "concurrent"); n != 1600 {
		t.Errorf("wrote %d records, want 1600", n)
	}
}

func TestNewClampsNonPositiveQueue(t *testing.T) {
	var out syncBuf
	for _, q := range []int{0, -1, -1000} {
		lw := New(&out, q)
		if cap(lw.ch) != defaultQueue {
			t.Errorf("queue %d: cap %d, want %d",
				q, cap(lw.ch), defaultQueue)
		}
		_ = lw.Close()
	}
}
