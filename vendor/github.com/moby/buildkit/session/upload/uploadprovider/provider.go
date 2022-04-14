package uploadprovider

import (
	"io"
	"path"
	"sync"

	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session/upload"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func New() *Uploader {
	return &Uploader{m: map[string]io.Reader{}}
}

type Uploader struct {
	mu sync.Mutex
	m  map[string]io.Reader
}

func (hp *Uploader) Add(r io.Reader) string {
	id := identity.NewID()
	hp.m[id] = r
	return "http://buildkit-session/" + id
}

func (hp *Uploader) Register(server *grpc.Server) {
	upload.RegisterUploadServer(server, hp)
}

func (hp *Uploader) Pull(stream upload.Upload_PullServer) error {
	opts, _ := metadata.FromIncomingContext(stream.Context()) // if no metadata continue with empty object
	var p string
	urls, ok := opts["urlpath"]
	if ok && len(urls) > 0 {
		p = urls[0]
	}

	p = path.Base(p)

	hp.mu.Lock()
	r, ok := hp.m[p]
	if !ok {
		hp.mu.Unlock()
		return errors.Errorf("no http response from session for %s", p)
	}
	delete(hp.m, p)
	hp.mu.Unlock()

	_, err := io.Copy(&writer{stream}, r)
	return err
}

type writer struct {
	grpc.ServerStream
}

func (w *writer) Write(dt []byte) (int, error) {
	if err := w.SendMsg(&upload.BytesMessage{Data: dt}); err != nil {
		return 0, err
	}
	return len(dt), nil
}
