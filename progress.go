package oget

import (
	"io"
	"sync/atomic"
)

type ProgressPhase int

const (
	Downloading ProgressPhase = iota
	Coping
	Done
)

type ProgressListener func(event ProgressEvent)
type ProgressEvent struct {
	Phase    ProgressPhase
	Progress int64
	Total    int64
}

type progress struct {
	phase    ProgressPhase
	length   int64
	progress int64
	handler  func(event ProgressEvent)
}

func downloadingProgress(length int64, handler func(event ProgressEvent)) *progress {
	return &progress{
		phase:    Downloading,
		length:   length,
		handler:  handler,
		progress: 0,
	}
}

func (p *progress) toCopingPhase() *progress {
	return &progress{
		phase:    Coping,
		length:   p.length,
		handler:  p.handler,
		progress: 0,
	}
}

func (p *progress) reader(proxy io.Reader) io.Reader {
	return &progressReader{parent: p, proxy: proxy}
}

func (p *progress) fireDone() {
	p.handler(ProgressEvent{
		Phase:    Done,
		Total:    p.length,
		Progress: p.length,
	})
}

type progressReader struct {
	parent *progress
	proxy  io.Reader
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.proxy.Read(p)
	if err != nil {
		return n, err
	}
	parent := r.parent
	progress := atomic.AddInt64(&parent.progress, int64(n))
	r.parent.handler(ProgressEvent{
		Phase:    parent.phase,
		Total:    parent.length,
		Progress: progress,
	})
	return n, err
}
