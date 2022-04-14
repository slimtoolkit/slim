package progress

import (
	"strings"

	"github.com/moby/buildkit/client"
)

func WithPrefix(w Writer, pfx string, force bool) Writer {
	return &prefixed{
		Writer: w,
		pfx:    pfx,
		force:  force,
	}
}

type prefixed struct {
	Writer
	pfx   string
	force bool
}

func (p *prefixed) Write(v *client.SolveStatus) {
	if p.force {
		for _, v := range v.Vertexes {
			v.Name = addPrefix(p.pfx, v.Name)
		}
	}
	p.Writer.Write(v)
}

func addPrefix(pfx, name string) string {
	if strings.HasPrefix(name, "[") {
		return "[" + pfx + " " + name[1:]
	}
	return "[" + pfx + "] " + name
}
