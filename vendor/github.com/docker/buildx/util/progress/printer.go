package progress

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/containerd/console"
	"github.com/docker/buildx/util/logutil"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

const (
	PrinterModeAuto  = "auto"
	PrinterModeTty   = "tty"
	PrinterModePlain = "plain"
	PrinterModeQuiet = "quiet"
)

type Printer struct {
	status       chan *client.SolveStatus
	done         <-chan struct{}
	err          error
	warnings     []client.VertexWarning
	logMu        sync.Mutex
	logSourceMap map[digest.Digest]interface{}
}

func (p *Printer) Wait() error {
	close(p.status)
	<-p.done
	return p.err
}

func (p *Printer) Write(s *client.SolveStatus) {
	p.status <- s
}

func (p *Printer) Warnings() []client.VertexWarning {
	return p.warnings
}

func (p *Printer) ValidateLogSource(dgst digest.Digest, v interface{}) bool {
	p.logMu.Lock()
	defer p.logMu.Unlock()
	src, ok := p.logSourceMap[dgst]
	if ok {
		if src == v {
			return true
		}
	} else {
		p.logSourceMap[dgst] = v
		return true
	}
	return false
}

func (p *Printer) ClearLogSource(v interface{}) {
	p.logMu.Lock()
	defer p.logMu.Unlock()
	for d := range p.logSourceMap {
		if p.logSourceMap[d] == v {
			delete(p.logSourceMap, d)
		}
	}
}

func NewPrinter(ctx context.Context, w io.Writer, out console.File, mode string) *Printer {
	statusCh := make(chan *client.SolveStatus)
	doneCh := make(chan struct{})

	pw := &Printer{
		status:       statusCh,
		done:         doneCh,
		logSourceMap: map[digest.Digest]interface{}{},
	}

	if v := os.Getenv("BUILDKIT_PROGRESS"); v != "" && mode == PrinterModeAuto {
		mode = v
	}

	go func() {
		var c console.Console
		switch mode {
		case PrinterModeQuiet:
			w = ioutil.Discard
		case PrinterModeAuto, PrinterModeTty:
			if cons, err := console.ConsoleFromFile(out); err == nil {
				c = cons
			}
		}
		resumeLogs := logutil.Pause(logrus.StandardLogger())
		// not using shared context to not disrupt display but let is finish reporting errors
		pw.warnings, pw.err = progressui.DisplaySolveStatus(ctx, "", c, w, statusCh)
		resumeLogs()
		close(doneCh)
	}()
	return pw
}
