package core

import "io"

// Progress is a snapshot of an ongoing long operation (download, extract, ...).
// Total may be 0 if the size is unknown — the UI should fall back to an
// indeterminate spinner in that case.
type Progress struct {
	Active     bool   `json:"active"`
	Stage      string `json:"stage"`              // human-readable: "Downloading xray.exe"
	File       string `json:"file,omitempty"`     // current file name
	Downloaded int64  `json:"downloaded"`         // bytes done in current step
	Total      int64  `json:"total"`              // total bytes for current step (0 = unknown)
	Step       int    `json:"step,omitempty"`     // current step in a multi-step task
	StepCount  int    `json:"stepCount,omitempty"`
}

// ProgressFn is the callback used by long operations to report progress.
type ProgressFn func(p Progress)

// progressReader wraps an io.Reader and reports bytes read through a callback.
// Throttled internally so we don't fire 1000 events per second on fast links.
type progressReader struct {
	r        io.Reader
	total    int64
	read     int64
	stage    string
	file     string
	step     int
	stepCnt  int
	cb       ProgressFn
	threshold int64 // emit when read - lastEmit >= threshold
	lastEmit int64
}

func newProgressReader(r io.Reader, total int64, stage, file string, step, stepCount int, cb ProgressFn) *progressReader {
	threshold := total / 50 // ~50 updates per file
	if threshold < 16*1024 {
		threshold = 16 * 1024
	}
	return &progressReader{
		r:         r,
		total:     total,
		stage:     stage,
		file:      file,
		step:      step,
		stepCnt:   stepCount,
		cb:        cb,
		threshold: threshold,
	}
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	p.read += int64(n)
	if p.cb != nil && (p.read-p.lastEmit >= p.threshold || err == io.EOF) {
		p.lastEmit = p.read
		p.cb(Progress{
			Active:     true,
			Stage:      p.stage,
			File:       p.file,
			Downloaded: p.read,
			Total:      p.total,
			Step:       p.step,
			StepCount:  p.stepCnt,
		})
	}
	return n, err
}
