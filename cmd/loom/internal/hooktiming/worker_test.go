package hooktiming

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/jsonlstore"
)

const workerFileBytes = 20 << 10

// worker owns a few files and changes them between calls, so every call of
// the concurrency scenario finds hunks that changed.
type worker struct {
	p       *project
	files   []string
	counter int
	sink    capture.EventSink
}

func (p *project) worker(index, fileCount int) *worker {
	w := &worker{p: p, sink: jsonlstore.NewDaySink(p.layout)}
	for i := range fileCount {
		name := fmt.Sprintf("work/w%d_f%d.go", index, i)
		p.writeSource(name, workerFileBytes, "")
		w.files = append(w.files, name)
	}
	return w
}

// changeAndRecord records a fresh agent write for the current content of each
// file in one batch, then rewrites the files so that hash no longer matches.
func (w *worker) changeAndRecord() {
	w.counter++
	var batch []capture.SessionEvent
	for _, name := range w.files {
		batch = append(batch, w.p.hunkEvent(name, w.p.readSource(name), time.Now(), hunkFirstLine, hunkLastLine))
		w.p.writeSource(name, workerFileBytes, fmt.Sprintf("CHANGE-%d", w.counter))
	}
	if err := w.sink.Append(context.Background(), batch); err != nil {
		w.p.t.Error(err)
	}
}

func (p *project) readSource(relative string) string {
	body, err := os.ReadFile(filepath.Join(p.root, relative))
	if err != nil {
		p.t.Error(err)
	}
	return string(body)
}
