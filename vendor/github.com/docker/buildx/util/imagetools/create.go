package imagetools

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func (r *Resolver) Combine(ctx context.Context, in string, descs []ocispec.Descriptor) ([]byte, ocispec.Descriptor, error) {
	ref, err := parseRef(in)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	eg, ctx := errgroup.WithContext(ctx)

	dts := make([][]byte, len(descs))
	for i := range dts {
		func(i int) {
			eg.Go(func() error {
				dt, err := r.GetDescriptor(ctx, ref.String(), descs[i])
				if err != nil {
					return err
				}
				dts[i] = dt

				if descs[i].MediaType == "" {
					mt, err := detectMediaType(dt)
					if err != nil {
						return err
					}
					descs[i].MediaType = mt
				}

				mt := descs[i].MediaType

				switch mt {
				case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
					p := descs[i].Platform
					if descs[i].Platform == nil {
						p = &ocispec.Platform{}
					}
					if p.OS == "" || p.Architecture == "" {
						if err := r.loadPlatform(ctx, p, in, dt); err != nil {
							return err
						}
					}
					descs[i].Platform = p
				case images.MediaTypeDockerSchema1Manifest:
					return errors.Errorf("schema1 manifests are not allowed in manifest lists")
				}

				return nil
			})
		}(i)
	}

	if err := eg.Wait(); err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	// on single source, return original bytes
	if len(descs) == 1 {
		if mt := descs[0].MediaType; mt == images.MediaTypeDockerSchema2ManifestList || mt == ocispec.MediaTypeImageIndex {
			return dts[0], descs[0], nil
		}
	}

	m := map[digest.Digest]int{}
	newDescs := make([]ocispec.Descriptor, 0, len(descs))

	addDesc := func(d ocispec.Descriptor) {
		idx, ok := m[d.Digest]
		if ok {
			old := newDescs[idx]
			if old.MediaType == "" {
				old.MediaType = d.MediaType
			}
			if d.Platform != nil {
				old.Platform = d.Platform
			}
			if old.Annotations == nil {
				old.Annotations = map[string]string{}
			}
			for k, v := range d.Annotations {
				old.Annotations[k] = v
			}
			newDescs[idx] = old
		} else {
			m[d.Digest] = len(newDescs)
			newDescs = append(newDescs, d)
		}
	}

	for i, desc := range descs {
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
			var mfst ocispec.Index
			if err := json.Unmarshal(dts[i], &mfst); err != nil {
				return nil, ocispec.Descriptor{}, errors.WithStack(err)
			}
			for _, d := range mfst.Manifests {
				addDesc(d)
			}
		default:
			addDesc(desc)
		}
	}

	mt := images.MediaTypeDockerSchema2ManifestList //ocispec.MediaTypeImageIndex
	idx := struct {
		// MediaType is reserved in the OCI spec but
		// excluded from go types.
		MediaType string `json:"mediaType,omitempty"`

		ocispec.Index
	}{
		MediaType: mt,
		Index: ocispec.Index{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			Manifests: newDescs,
		},
	}

	idxBytes, err := json.MarshalIndent(idx, "", "   ")
	if err != nil {
		return nil, ocispec.Descriptor{}, errors.Wrap(err, "failed to marshal index")
	}

	return idxBytes, ocispec.Descriptor{
		MediaType: mt,
		Size:      int64(len(idxBytes)),
		Digest:    digest.FromBytes(idxBytes),
	}, nil
}

func (r *Resolver) Push(ctx context.Context, ref reference.Named, desc ocispec.Descriptor, dt []byte) error {
	ref = reference.TagNameOnly(ref)

	p, err := r.resolver().Pusher(ctx, ref.String())
	if err != nil {
		return err
	}
	cw, err := p.Push(ctx, desc)
	if err != nil {
		if errdefs.IsAlreadyExists(err) {
			return nil
		}
		return err
	}

	err = content.Copy(ctx, cw, bytes.NewReader(dt), desc.Size, desc.Digest)
	if errdefs.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (r *Resolver) loadPlatform(ctx context.Context, p2 *ocispec.Platform, in string, dt []byte) error {
	var manifest ocispec.Manifest
	if err := json.Unmarshal(dt, &manifest); err != nil {
		return errors.WithStack(err)
	}

	dt, err := r.GetDescriptor(ctx, in, manifest.Config)
	if err != nil {
		return err
	}

	var p ocispec.Platform
	if err := json.Unmarshal(dt, &p); err != nil {
		return errors.WithStack(err)
	}

	p = platforms.Normalize(p)

	if p2.Architecture == "" {
		p2.Architecture = p.Architecture
		if p2.Variant == "" {
			p2.Variant = p.Variant
		}
	}
	if p2.OS == "" {
		p2.OS = p.OS
	}

	return nil
}

func detectMediaType(dt []byte) (string, error) {
	var mfst struct {
		MediaType string          `json:"mediaType"`
		Config    json.RawMessage `json:"config"`
		FSLayers  []string        `json:"fsLayers"`
	}

	if err := json.Unmarshal(dt, &mfst); err != nil {
		return "", errors.WithStack(err)
	}

	if mfst.MediaType != "" {
		return mfst.MediaType, nil
	}
	if mfst.Config != nil {
		return images.MediaTypeDockerSchema2Manifest, nil
	}
	if len(mfst.FSLayers) > 0 {
		return images.MediaTypeDockerSchema1Manifest, nil
	}

	return images.MediaTypeDockerSchema2ManifestList, nil
}
