// Package logwriter provides a non-blocking io.Writer wrapper.
//
// Inside a container os.Stdout and os.Stderr are pipes to the runtime's
// log collector, with a fixed kernel buffer (64KB on Linux). When the
// collector lags -- during a burst of output, or while the log file is
// being rotated -- write(2) on that pipe blocks.
//
// That is not survivable from a request handler. net/http buffers a
// response and only flushes it to the socket once the handler chain
// returns, so a handler blocked while logging holds an already-written
// response that the client never sees: the connection was accepted and
// then simply never answered. Health probes read that as a timeout and
// restart the container.
//
// Writer hands each record to a background goroutine and returns
// immediately, so only that goroutine can ever block on the pipe. When
// the queue is full, records are dropped and counted, and the count is
// reported once the writer drains. Losing log lines is strictly better
// than stalling requests.
package logwriter

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

const (
	// defaultQueue is how many records are held before dropping. At
	// roughly 200 bytes per access log line this is a few MB of
	// headroom, enough to ride out a log rotation without dropping.
	defaultQueue = 8192

	// batchBytes caps how much the writer coalesces into a single
	// write syscall. A burst of requests then costs one write rather
	// than one per line, which is itself part of why the pipe fills.
	batchBytes = 64 << 10
)

// Writer forwards records to an underlying io.Writer from a background
// goroutine. It is safe for concurrent use.
type Writer struct {
	ch   chan []byte
	stop chan struct{}
	done chan struct{}
	once sync.Once

	// dropped counts records discarded because the queue was full.
	// reported is the portion already announced in the output, and is
	// only ever touched by the run goroutine.
	dropped  atomic.Uint64
	reported uint64
}

// New returns a Writer forwarding to w from a background goroutine.
// queue is the number of records held before dropping; zero or less
// uses a default. Close flushes what is queued and stops the goroutine.
func New(w io.Writer, queue int) *Writer {
	if queue <= 0 {
		queue = defaultQueue
	}
	lw := &Writer{
		ch:   make(chan []byte, queue),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go lw.run(w)
	return lw
}

// Write queues p and never blocks. io.Writer forbids retaining p once
// Write returns, so the record is copied first. A full queue drops the
// record and increments the drop counter rather than stalling the
// caller, and the error is never surfaced: a caller that cannot afford
// to block also cannot do anything useful with a write failure.
func (lw *Writer) Write(p []byte) (int, error) {
	rec := make([]byte, len(p))
	copy(rec, p)
	select {
	case lw.ch <- rec:
	default:
		lw.dropped.Add(1)
	}
	return len(p), nil
}

// Close flushes queued records and stops the background goroutine.
// Writes after Close are dropped rather than panicking, since handlers
// can still be in flight during shutdown.
func (lw *Writer) Close() error {
	lw.once.Do(func() { close(lw.stop) })
	<-lw.done
	return nil
}

// Dropped reports how many records have been discarded because the
// queue was full.
func (lw *Writer) Dropped() uint64 { return lw.dropped.Load() }

func (lw *Writer) run(w io.Writer) {
	defer close(lw.done)
	var buf []byte
	for {
		select {
		case rec := <-lw.ch:
			buf = lw.batch(append(buf[:0], rec...))
			_, _ = w.Write(buf)
		case <-lw.stop:
			// Drain whatever is still queued so a graceful
			// shutdown does not silently discard it.
			buf = lw.batch(buf[:0])
			if len(buf) > 0 {
				_, _ = w.Write(buf)
			}
			return
		}
	}
}

// batch appends every record already queued, up to batchBytes, then a
// notice for anything dropped since the last write.
func (lw *Writer) batch(buf []byte) []byte {
	for len(buf) < batchBytes {
		select {
		case rec := <-lw.ch:
			buf = append(buf, rec...)
		default:
			return lw.appendDropped(buf)
		}
	}
	return lw.appendDropped(buf)
}

func (lw *Writer) appendDropped(buf []byte) []byte {
	total := lw.dropped.Load()
	if total == lw.reported {
		return buf
	}
	n := total - lw.reported
	lw.reported = total
	return append(buf, fmt.Sprintf(
		"log writer dropped %d records while stalled\n", n)...)
}
