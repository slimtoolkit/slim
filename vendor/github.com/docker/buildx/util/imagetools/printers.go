package imagetools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	binfotypes "github.com/moby/buildkit/util/buildinfo/types"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const defaultPfx = "  "

type Printer struct {
	ctx      context.Context
	resolver *Resolver

	name   string
	format string

	raw       []byte
	ref       reference.Named
	manifest  ocispecs.Descriptor
	index     ocispecs.Index
	platforms []ocispecs.Platform
}

func NewPrinter(ctx context.Context, opt Opt, name string, format string) (*Printer, error) {
	resolver := New(opt)

	ref, err := parseRef(name)
	if err != nil {
		return nil, err
	}

	dt, manifest, err := resolver.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	var index ocispecs.Index
	if err = json.Unmarshal(dt, &index); err != nil {
		return nil, err
	}

	var pforms []ocispecs.Platform
	switch manifest.MediaType {
	case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
		for _, m := range index.Manifests {
			pforms = append(pforms, *m.Platform)
		}
	default:
		pforms = append(pforms, platforms.DefaultSpec())
	}

	return &Printer{
		ctx:       ctx,
		resolver:  resolver,
		name:      name,
		format:    format,
		raw:       dt,
		ref:       ref,
		manifest:  manifest,
		index:     index,
		platforms: pforms,
	}, nil
}

func (p *Printer) Print(raw bool, out io.Writer) error {
	if raw {
		_, err := fmt.Fprintf(out, "%s", p.raw) // avoid newline to keep digest
		return err
	}

	if p.format == "" {
		w := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
		_, _ = fmt.Fprintf(w, "Name:\t%s\n", p.ref.String())
		_, _ = fmt.Fprintf(w, "MediaType:\t%s\n", p.manifest.MediaType)
		_, _ = fmt.Fprintf(w, "Digest:\t%s\n", p.manifest.Digest)
		_ = w.Flush()
		switch p.manifest.MediaType {
		case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
			if err := p.printManifestList(out); err != nil {
				return err
			}
		}
		return nil
	}

	tpl, err := template.New("").Funcs(template.FuncMap{
		"json": func(v interface{}) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		},
	}).Parse(p.format)
	if err != nil {
		return err
	}

	imageconfigs := make(map[string]*ocispecs.Image)
	buildinfos := make(map[string]*binfotypes.BuildInfo)

	eg, _ := errgroup.WithContext(p.ctx)
	for _, platform := range p.platforms {
		func(platform ocispecs.Platform) {
			eg.Go(func() error {
				img, dtic, err := p.getImageConfig(&platform)
				if err != nil {
					return err
				} else if img != nil {
					imageconfigs[platforms.Format(platform)] = img
				}
				if bi, err := p.getBuildInfo(dtic); err != nil {
					return err
				} else if bi != nil {
					buildinfos[platforms.Format(platform)] = bi
				}
				return nil
			})
		}(platform)
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	format := tpl.Root.String()

	var manifest interface{}
	switch p.manifest.MediaType {
	case images.MediaTypeDockerSchema2Manifest, ocispecs.MediaTypeImageManifest:
		manifest = p.manifest
	case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
		manifest = struct {
			SchemaVersion int                   `json:"schemaVersion"`
			MediaType     string                `json:"mediaType,omitempty"`
			Digest        digest.Digest         `json:"digest"`
			Size          int64                 `json:"size"`
			Manifests     []ocispecs.Descriptor `json:"manifests"`
			Annotations   map[string]string     `json:"annotations,omitempty"`
		}{
			SchemaVersion: p.index.Versioned.SchemaVersion,
			MediaType:     p.index.MediaType,
			Digest:        p.manifest.Digest,
			Size:          p.manifest.Size,
			Manifests:     p.index.Manifests,
			Annotations:   p.index.Annotations,
		}
	}

	switch {
	// TODO: print formatted config
	case strings.HasPrefix(format, "{{.Manifest"), strings.HasPrefix(format, "{{.BuildInfo"):
		w := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
		_, _ = fmt.Fprintf(w, "Name:\t%s\n", p.ref.String())
		if strings.HasPrefix(format, "{{.Manifest") {
			_, _ = fmt.Fprintf(w, "MediaType:\t%s\n", p.manifest.MediaType)
			_, _ = fmt.Fprintf(w, "Digest:\t%s\n", p.manifest.Digest)
			_ = w.Flush()
			switch p.manifest.MediaType {
			case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
				_ = p.printManifestList(out)
			}
		} else if strings.HasPrefix(format, "{{.BuildInfo") {
			_ = w.Flush()
			_ = p.printBuildInfos(buildinfos, out)
		}
	default:
		if len(p.platforms) > 1 {
			return tpl.Execute(out, struct {
				Name      string                           `json:"name,omitempty"`
				Manifest  interface{}                      `json:"manifest,omitempty"`
				Image     map[string]*ocispecs.Image       `json:"image,omitempty"`
				BuildInfo map[string]*binfotypes.BuildInfo `json:"buildinfo,omitempty"`
			}{
				Name:      p.name,
				Manifest:  manifest,
				Image:     imageconfigs,
				BuildInfo: buildinfos,
			})
		}
		var ic *ocispecs.Image
		for _, v := range imageconfigs {
			ic = v
		}
		var bi *binfotypes.BuildInfo
		for _, v := range buildinfos {
			bi = v
		}
		return tpl.Execute(out, struct {
			Name      string                `json:"name,omitempty"`
			Manifest  interface{}           `json:"manifest,omitempty"`
			Image     *ocispecs.Image       `json:"image,omitempty"`
			BuildInfo *binfotypes.BuildInfo `json:"buildinfo,omitempty"`
		}{
			Name:      p.name,
			Manifest:  manifest,
			Image:     ic,
			BuildInfo: bi,
		})
	}

	return nil
}

func (p *Printer) printManifestList(out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
	_, _ = fmt.Fprintf(w, "\t\n")
	_, _ = fmt.Fprintf(w, "Manifests:\t\n")
	_ = w.Flush()

	w = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for i, m := range p.index.Manifests {
		if i != 0 {
			_, _ = fmt.Fprintf(w, "\t\n")
		}
		cr, err := reference.WithDigest(p.ref, m.Digest)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "%sName:\t%s\n", defaultPfx, cr.String())
		_, _ = fmt.Fprintf(w, "%sMediaType:\t%s\n", defaultPfx, m.MediaType)
		if p := m.Platform; p != nil {
			_, _ = fmt.Fprintf(w, "%sPlatform:\t%s\n", defaultPfx, platforms.Format(*p))
			if p.OSVersion != "" {
				_, _ = fmt.Fprintf(w, "%sOSVersion:\t%s\n", defaultPfx, p.OSVersion)
			}
			if len(p.OSFeatures) > 0 {
				_, _ = fmt.Fprintf(w, "%sOSFeatures:\t%s\n", defaultPfx, strings.Join(p.OSFeatures, ", "))
			}
			if len(m.URLs) > 0 {
				_, _ = fmt.Fprintf(w, "%sURLs:\t%s\n", defaultPfx, strings.Join(m.URLs, ", "))
			}
			if len(m.Annotations) > 0 {
				_ = w.Flush()
				w2 := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
				for k, v := range m.Annotations {
					_, _ = fmt.Fprintf(w2, "%s%s:\t%s\n", defaultPfx+defaultPfx, k, v)
				}
				_ = w2.Flush()
			}
		}
	}
	return w.Flush()
}

func (p *Printer) printBuildInfos(bis map[string]*binfotypes.BuildInfo, out io.Writer) error {
	if len(bis) == 0 {
		return nil
	} else if len(bis) == 1 {
		for _, bi := range bis {
			return p.printBuildInfo(bi, "", out)
		}
	}
	var pkeys []string
	for _, pform := range p.platforms {
		pkeys = append(pkeys, platforms.Format(pform))
	}
	sort.Strings(pkeys)
	for _, platform := range pkeys {
		bi := bis[platform]
		w := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
		_, _ = fmt.Fprintf(w, "\t\nPlatform:\t%s\t\n", platform)
		_ = w.Flush()
		if err := p.printBuildInfo(bi, "", out); err != nil {
			return err
		}
	}
	return nil
}

func (p *Printer) printBuildInfo(bi *binfotypes.BuildInfo, pfx string, out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
	_, _ = fmt.Fprintf(w, "%sFrontend:\t%s\n", pfx, bi.Frontend)

	if len(bi.Attrs) > 0 {
		_, _ = fmt.Fprintf(w, "%sAttrs:\t\n", pfx)
		_ = w.Flush()
		for k, v := range bi.Attrs {
			_, _ = fmt.Fprintf(w, "%s%s:\t%s\n", pfx+defaultPfx, k, *v)
		}
	}

	if len(bi.Sources) > 0 {
		_, _ = fmt.Fprintf(w, "%sSources:\t\n", pfx)
		_ = w.Flush()
		for i, v := range bi.Sources {
			if i != 0 {
				_, _ = fmt.Fprintf(w, "\t\n")
			}
			_, _ = fmt.Fprintf(w, "%sType:\t%s\n", pfx+defaultPfx, v.Type)
			_, _ = fmt.Fprintf(w, "%sRef:\t%s\n", pfx+defaultPfx, v.Ref)
			_, _ = fmt.Fprintf(w, "%sPin:\t%s\n", pfx+defaultPfx, v.Pin)
		}
	}

	if len(bi.Deps) > 0 {
		_, _ = fmt.Fprintf(w, "%sDeps:\t\n", pfx)
		_ = w.Flush()
		firstPass := true
		for k, v := range bi.Deps {
			if !firstPass {
				_, _ = fmt.Fprintf(w, "\t\n")
			}
			_, _ = fmt.Fprintf(w, "%sName:\t%s\n", pfx+defaultPfx, k)
			_ = w.Flush()
			_ = p.printBuildInfo(&v, pfx+defaultPfx, out)
			firstPass = false
		}
	}

	return w.Flush()
}

func (p *Printer) getImageConfig(platform *ocispecs.Platform) (*ocispecs.Image, []byte, error) {
	_, dtic, err := p.resolver.ImageConfig(p.ctx, p.name, platform)
	if err != nil {
		return nil, nil, err
	}
	var img *ocispecs.Image
	if err = json.Unmarshal(dtic, &img); err != nil {
		return nil, nil, err
	}
	return img, dtic, nil
}

func (p *Printer) getBuildInfo(dtic []byte) (*binfotypes.BuildInfo, error) {
	var binfo *binfotypes.BuildInfo
	if len(dtic) > 0 {
		var biconfig binfotypes.ImageConfig
		if err := json.Unmarshal(dtic, &biconfig); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal image config")
		}
		if len(biconfig.BuildInfo) > 0 {
			dtbi, err := base64.StdEncoding.DecodeString(biconfig.BuildInfo)
			if err != nil {
				return nil, errors.Wrap(err, "failed to decode build info")
			}
			if err = json.Unmarshal(dtbi, &binfo); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal build info")
			}
		}
	}
	return binfo, nil
}
