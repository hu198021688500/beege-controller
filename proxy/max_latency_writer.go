package proxy

import (
	"io"
	"net/http"
	"sync"
	"time"
)

type writeFlusher interface {
	io.Writer
	http.Flusher
}

type maxLatencyWriter struct {
	dst     writeFlusher
	latency time.Duration

	wlk  sync.Mutex
	slk  sync.Mutex
	done chan bool
}

func NewMaxLatencyWriter(dst writeFlusher, latency time.Duration) *maxLatencyWriter {
	m := &maxLatencyWriter{
		dst:     dst,
		latency: latency,
		done:    make(chan bool),
	}

	go m.flushLoop(m.done)

	return m
}

func (m *maxLatencyWriter) Write(p []byte) (int, error) {
	m.wlk.Lock()
	defer m.wlk.Unlock()
	return m.dst.Write(p)
}

func (m *maxLatencyWriter) flushLoop(d chan bool) {
	t := time.NewTicker(m.latency)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			m.wlk.Lock()
			m.dst.Flush()
			m.wlk.Unlock()
		case <-d:
			return
		}
	}
	panic("unreached")
}

func (m *maxLatencyWriter) Stop() {
	m.slk.Lock()
	defer m.slk.Unlock()

	if m.done != nil {
		m.done <- true
		m.done = nil
	}
}
