package progresswriter

import (
	"time"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	digest "github.com/opencontainers/go-digest"
)

type Logger func(*client.SolveStatus)

type SubLogger interface {
	Wrap(name string, fn func() error) error
	Log(stream int, dt []byte)
}

func Wrap(name string, l Logger, fn func(SubLogger) error) (err error) {
	if l == nil {
		return nil
	}
	dgst := digest.FromBytes([]byte(identity.NewID()))
	tm := time.Now()
	l(&client.SolveStatus{
		Vertexes: []*client.Vertex{{
			Digest:  dgst,
			Name:    name,
			Started: &tm,
		}},
	})

	defer func() {
		tm2 := time.Now()
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		l(&client.SolveStatus{
			Vertexes: []*client.Vertex{{
				Digest:    dgst,
				Name:      name,
				Started:   &tm,
				Completed: &tm2,
				Error:     errMsg,
			}},
		})
	}()

	return fn(&subLogger{dgst, l})
}

type subLogger struct {
	dgst   digest.Digest
	logger Logger
}

func (sl *subLogger) Wrap(name string, fn func() error) (err error) {
	tm := time.Now()
	sl.logger(&client.SolveStatus{
		Statuses: []*client.VertexStatus{{
			Vertex:    sl.dgst,
			ID:        name,
			Timestamp: time.Now(),
			Started:   &tm,
		}},
	})

	defer func() {
		tm2 := time.Now()
		sl.logger(&client.SolveStatus{
			Statuses: []*client.VertexStatus{{
				Vertex:    sl.dgst,
				ID:        name,
				Timestamp: time.Now(),
				Started:   &tm,
				Completed: &tm2,
			}},
		})
	}()

	return fn()
}

func (sl *subLogger) Log(stream int, dt []byte) {
	sl.logger(&client.SolveStatus{
		Logs: []*client.VertexLog{{
			Vertex:    sl.dgst,
			Stream:    stream,
			Data:      dt,
			Timestamp: time.Now(),
		}},
	})
}
