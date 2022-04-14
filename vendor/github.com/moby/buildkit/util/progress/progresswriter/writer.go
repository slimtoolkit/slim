package progresswriter

import (
	"time"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	digest "github.com/opencontainers/go-digest"
)

type Writer interface {
	Done() <-chan struct{}
	Err() error
	Status() chan *client.SolveStatus
}

func Write(w Writer, name string, f func() error) {
	status := w.Status()
	dgst := digest.FromBytes([]byte(identity.NewID()))
	tm := time.Now()

	vtx := client.Vertex{
		Digest:  dgst,
		Name:    name,
		Started: &tm,
	}

	status <- &client.SolveStatus{
		Vertexes: []*client.Vertex{&vtx},
	}

	var err error
	if f != nil {
		err = f()
	}

	tm2 := time.Now()
	vtx2 := vtx
	vtx2.Completed = &tm2
	if err != nil {
		vtx2.Error = err.Error()
	}
	status <- &client.SolveStatus{
		Vertexes: []*client.Vertex{&vtx2},
	}
}
