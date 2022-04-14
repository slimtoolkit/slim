package build

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/buildx/driver"
	"github.com/docker/buildx/util/imagetools"
	"github.com/docker/buildx/util/progress"
	"github.com/docker/buildx/util/resolver"
	"github.com/docker/buildx/util/waitmap"
	"github.com/docker/cli/opts"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/upload/uploadprovider"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/moby/buildkit/util/tracing"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

var (
	errStdinConflict      = errors.New("invalid argument: can't use stdin for both build context and dockerfile")
	errDockerfileConflict = errors.New("ambiguous Dockerfile source: both stdin and flag correspond to Dockerfiles")
)

type Options struct {
	Inputs Inputs

	Allow         []entitlements.Entitlement
	BuildArgs     map[string]string
	CacheFrom     []client.CacheOptionsEntry
	CacheTo       []client.CacheOptionsEntry
	CgroupParent  string
	Exports       []client.ExportEntry
	ExtraHosts    []string
	ImageIDFile   string
	Labels        map[string]string
	NetworkMode   string
	NoCache       bool
	NoCacheFilter []string
	Platforms     []specs.Platform
	Pull          bool
	Session       []session.Attachable
	ShmSize       opts.MemBytes
	Tags          []string
	Target        string
	Ulimits       *opts.UlimitOpt
}

type Inputs struct {
	ContextPath      string
	DockerfilePath   string
	InStream         io.Reader
	ContextState     *llb.State
	DockerfileInline string
	NamedContexts    map[string]NamedContext
}

type NamedContext struct {
	Path  string
	State *llb.State
}

type DriverInfo struct {
	Driver      driver.Driver
	Name        string
	Platform    []specs.Platform
	Err         error
	ImageOpt    imagetools.Opt
	ProxyConfig map[string]string
}

type DockerAPI interface {
	DockerAPI(name string) (dockerclient.APIClient, error)
}

func filterAvailableDrivers(drivers []DriverInfo) ([]DriverInfo, error) {
	out := make([]DriverInfo, 0, len(drivers))
	err := errors.Errorf("no drivers found")
	for _, di := range drivers {
		if di.Err == nil && di.Driver != nil {
			out = append(out, di)
		}
		if di.Err != nil {
			err = di.Err
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	return nil, err
}

type driverPair struct {
	driverIndex int
	platforms   []specs.Platform
	so          *client.SolveOpt
	bopts       gateway.BuildOpts
}

func driverIndexes(m map[string][]driverPair) []int {
	out := make([]int, 0, len(m))
	visited := map[int]struct{}{}
	for _, dp := range m {
		for _, d := range dp {
			if _, ok := visited[d.driverIndex]; ok {
				continue
			}
			visited[d.driverIndex] = struct{}{}
			out = append(out, d.driverIndex)
		}
	}
	return out
}

func allIndexes(l int) []int {
	out := make([]int, 0, l)
	for i := 0; i < l; i++ {
		out = append(out, i)
	}
	return out
}

func ensureBooted(ctx context.Context, drivers []DriverInfo, idxs []int, pw progress.Writer) ([]*client.Client, error) {
	clients := make([]*client.Client, len(drivers))

	baseCtx := ctx
	eg, ctx := errgroup.WithContext(ctx)

	for _, i := range idxs {
		func(i int) {
			eg.Go(func() error {
				c, err := driver.Boot(ctx, baseCtx, drivers[i].Driver, pw)
				if err != nil {
					return err
				}
				clients[i] = c
				return nil
			})
		}(i)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return clients, nil
}

func splitToDriverPairs(availablePlatforms map[string]int, opt map[string]Options) map[string][]driverPair {
	m := map[string][]driverPair{}
	for k, opt := range opt {
		mm := map[int][]specs.Platform{}
		for _, p := range opt.Platforms {
			k := platforms.Format(p)
			idx := availablePlatforms[k] // default 0
			pp := mm[idx]
			pp = append(pp, p)
			mm[idx] = pp
		}
		// if no platform is specified, use first driver
		if len(mm) == 0 {
			mm[0] = nil
		}
		dps := make([]driverPair, 0, 2)
		for idx, pp := range mm {
			dps = append(dps, driverPair{driverIndex: idx, platforms: pp})
		}
		m[k] = dps
	}
	return m
}

func resolveDrivers(ctx context.Context, drivers []DriverInfo, opt map[string]Options, pw progress.Writer) (map[string][]driverPair, []*client.Client, error) {
	dps, clients, err := resolveDriversBase(ctx, drivers, opt, pw)
	if err != nil {
		return nil, nil, err
	}

	bopts := make([]gateway.BuildOpts, len(clients))

	span, ctx := tracing.StartSpan(ctx, "load buildkit capabilities", trace.WithSpanKind(trace.SpanKindInternal))

	eg, ctx := errgroup.WithContext(ctx)
	for i, c := range clients {
		if c == nil {
			continue
		}

		func(i int, c *client.Client) {
			eg.Go(func() error {
				clients[i].Build(ctx, client.SolveOpt{}, "buildx", func(ctx context.Context, c gateway.Client) (*gateway.Result, error) {
					bopts[i] = c.BuildOpts()
					return nil, nil
				}, nil)
				return nil
			})
		}(i, c)
	}

	err = eg.Wait()
	tracing.FinishWithError(span, err)
	if err != nil {
		return nil, nil, err
	}
	for key := range dps {
		for i, dp := range dps[key] {
			dps[key][i].bopts = bopts[dp.driverIndex]
		}
	}

	return dps, clients, nil
}

func resolveDriversBase(ctx context.Context, drivers []DriverInfo, opt map[string]Options, pw progress.Writer) (map[string][]driverPair, []*client.Client, error) {
	availablePlatforms := map[string]int{}
	for i, d := range drivers {
		for _, p := range d.Platform {
			availablePlatforms[platforms.Format(p)] = i
		}
	}

	undetectedPlatform := false
	allPlatforms := map[string]int{}
	for _, opt := range opt {
		for _, p := range opt.Platforms {
			k := platforms.Format(p)
			allPlatforms[k] = -1
			if _, ok := availablePlatforms[k]; !ok {
				undetectedPlatform = true
			}
		}
	}

	// fast path
	if len(drivers) == 1 || len(allPlatforms) == 0 {
		m := map[string][]driverPair{}
		for k, opt := range opt {
			m[k] = []driverPair{{driverIndex: 0, platforms: opt.Platforms}}
		}
		clients, err := ensureBooted(ctx, drivers, driverIndexes(m), pw)
		if err != nil {
			return nil, nil, err
		}
		return m, clients, nil
	}

	// map based on existing platforms
	if !undetectedPlatform {
		m := splitToDriverPairs(availablePlatforms, opt)
		clients, err := ensureBooted(ctx, drivers, driverIndexes(m), pw)
		if err != nil {
			return nil, nil, err
		}
		return m, clients, nil
	}

	// boot all drivers in k
	clients, err := ensureBooted(ctx, drivers, allIndexes(len(drivers)), pw)
	if err != nil {
		return nil, nil, err
	}

	eg, ctx := errgroup.WithContext(ctx)
	workers := make([][]*client.WorkerInfo, len(clients))

	for i, c := range clients {
		if c == nil {
			continue
		}
		func(i int) {
			eg.Go(func() error {
				ww, err := clients[i].ListWorkers(ctx)
				if err != nil {
					return errors.Wrap(err, "listing workers")
				}
				workers[i] = ww
				return nil
			})

		}(i)
	}

	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}

	for i, ww := range workers {
		for _, w := range ww {
			for _, p := range w.Platforms {
				p = platforms.Normalize(p)
				ps := platforms.Format(p)

				if _, ok := availablePlatforms[ps]; !ok {
					availablePlatforms[ps] = i
				}
			}
		}
	}

	return splitToDriverPairs(availablePlatforms, opt), clients, nil
}

func toRepoOnly(in string) (string, error) {
	m := map[string]struct{}{}
	p := strings.Split(in, ",")
	for _, pp := range p {
		n, err := reference.ParseNormalizedNamed(pp)
		if err != nil {
			return "", err
		}
		m[n.Name()] = struct{}{}
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return strings.Join(out, ","), nil
}

func toSolveOpt(ctx context.Context, di DriverInfo, multiDriver bool, opt Options, bopts gateway.BuildOpts, configDir string, pw progress.Writer, dl dockerLoadCallback) (solveOpt *client.SolveOpt, release func(), err error) {
	d := di.Driver
	defers := make([]func(), 0, 2)
	releaseF := func() {
		for _, f := range defers {
			f()
		}
	}

	defer func() {
		if err != nil {
			releaseF()
		}
	}()

	if opt.ImageIDFile != "" {
		// Avoid leaving a stale file if we eventually fail
		if err := os.Remove(opt.ImageIDFile); err != nil && !os.IsNotExist(err) {
			return nil, nil, errors.Wrap(err, "removing image ID file")
		}
	}

	// inline cache from build arg
	if v, ok := opt.BuildArgs["BUILDKIT_INLINE_CACHE"]; ok {
		if v, _ := strconv.ParseBool(v); v {
			opt.CacheTo = append(opt.CacheTo, client.CacheOptionsEntry{
				Type:  "inline",
				Attrs: map[string]string{},
			})
		}
	}

	for _, e := range opt.CacheTo {
		if e.Type != "inline" && !d.Features()[driver.CacheExport] {
			return nil, nil, notSupported(d, driver.CacheExport)
		}
	}

	cacheTo := make([]client.CacheOptionsEntry, 0, len(opt.CacheTo))
	for _, e := range opt.CacheTo {
		if e.Type == "gha" {
			if !bopts.LLBCaps.Contains(apicaps.CapID("cache.gha")) {
				continue
			}
		}
		cacheTo = append(cacheTo, e)
	}

	cacheFrom := make([]client.CacheOptionsEntry, 0, len(opt.CacheFrom))
	for _, e := range opt.CacheFrom {
		if e.Type == "gha" {
			if !bopts.LLBCaps.Contains(apicaps.CapID("cache.gha")) {
				continue
			}
		}
		cacheFrom = append(cacheFrom, e)
	}

	so := client.SolveOpt{
		Frontend:            "dockerfile.v0",
		FrontendAttrs:       map[string]string{},
		LocalDirs:           map[string]string{},
		CacheExports:        cacheTo,
		CacheImports:        cacheFrom,
		AllowedEntitlements: opt.Allow,
	}

	if opt.CgroupParent != "" {
		so.FrontendAttrs["cgroup-parent"] = opt.CgroupParent
	}

	if v, ok := opt.BuildArgs["BUILDKIT_MULTI_PLATFORM"]; ok {
		if v, _ := strconv.ParseBool(v); v {
			so.FrontendAttrs["multi-platform"] = "true"
		}
	}

	if multiDriver {
		// force creation of manifest list
		so.FrontendAttrs["multi-platform"] = "true"
	}

	switch len(opt.Exports) {
	case 1:
		// valid
	case 0:
		if d.IsMobyDriver() && !noDefaultLoad() {
			// backwards compat for docker driver only:
			// this ensures the build results in a docker image.
			opt.Exports = []client.ExportEntry{{Type: "image", Attrs: map[string]string{}}}
		}
	default:
		return nil, nil, errors.Errorf("multiple outputs currently unsupported")
	}

	// fill in image exporter names from tags
	if len(opt.Tags) > 0 {
		tags := make([]string, len(opt.Tags))
		for i, tag := range opt.Tags {
			ref, err := reference.Parse(tag)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "invalid tag %q", tag)
			}
			tags[i] = ref.String()
		}
		for i, e := range opt.Exports {
			switch e.Type {
			case "image", "oci", "docker":
				opt.Exports[i].Attrs["name"] = strings.Join(tags, ",")
			}
		}
	} else {
		for _, e := range opt.Exports {
			if e.Type == "image" && e.Attrs["name"] == "" && e.Attrs["push"] != "" {
				if ok, _ := strconv.ParseBool(e.Attrs["push"]); ok {
					return nil, nil, errors.Errorf("tag is needed when pushing to registry")
				}
			}
		}
	}

	// cacheonly is a fake exporter to opt out of default behaviors
	exports := make([]client.ExportEntry, 0, len(opt.Exports))
	for _, e := range opt.Exports {
		if e.Type != "cacheonly" {
			exports = append(exports, e)
		}
	}
	opt.Exports = exports

	// set up exporters
	for i, e := range opt.Exports {
		if (e.Type == "local" || e.Type == "tar") && opt.ImageIDFile != "" {
			return nil, nil, errors.Errorf("local and tar exporters are incompatible with image ID file")
		}
		if e.Type == "oci" && !d.Features()[driver.OCIExporter] {
			return nil, nil, notSupported(d, driver.OCIExporter)
		}
		if e.Type == "docker" {
			if len(opt.Platforms) > 1 {
				return nil, nil, errors.Errorf("docker exporter does not currently support exporting manifest lists")
			}
			if e.Output == nil {
				if d.IsMobyDriver() {
					e.Type = "image"
				} else {
					w, cancel, err := dl(e.Attrs["context"])
					if err != nil {
						return nil, nil, err
					}
					defers = append(defers, cancel)
					opt.Exports[i].Output = wrapWriteCloser(w)
				}
			} else if !d.Features()[driver.DockerExporter] {
				return nil, nil, notSupported(d, driver.DockerExporter)
			}
		}
		if e.Type == "image" && d.IsMobyDriver() {
			opt.Exports[i].Type = "moby"
			if e.Attrs["push"] != "" {
				if ok, _ := strconv.ParseBool(e.Attrs["push"]); ok {
					if ok, _ := strconv.ParseBool(e.Attrs["push-by-digest"]); ok {
						return nil, nil, errors.Errorf("push-by-digest is currently not implemented for docker driver, please create a new builder instance")
					}
				}
			}
		}
		if e.Type == "docker" || e.Type == "image" || e.Type == "oci" {
			// inline buildinfo attrs from build arg
			if v, ok := opt.BuildArgs["BUILDKIT_INLINE_BUILDINFO_ATTRS"]; ok {
				e.Attrs["buildinfo-attrs"] = v
			}
		}
	}

	so.Exports = opt.Exports
	so.Session = opt.Session

	releaseLoad, err := LoadInputs(ctx, d, opt.Inputs, pw, &so)
	if err != nil {
		return nil, nil, err
	}
	defers = append(defers, releaseLoad)

	if sharedKey := so.LocalDirs["context"]; sharedKey != "" {
		if p, err := filepath.Abs(sharedKey); err == nil {
			sharedKey = filepath.Base(p)
		}
		so.SharedKey = sharedKey + ":" + tryNodeIdentifier(configDir)
	}

	if opt.Pull {
		so.FrontendAttrs["image-resolve-mode"] = "pull"
	}
	if opt.Target != "" {
		so.FrontendAttrs["target"] = opt.Target
	}
	if len(opt.NoCacheFilter) > 0 {
		so.FrontendAttrs["no-cache"] = strings.Join(opt.NoCacheFilter, ",")
	}
	if opt.NoCache {
		so.FrontendAttrs["no-cache"] = ""
	}
	for k, v := range opt.BuildArgs {
		so.FrontendAttrs["build-arg:"+k] = v
	}
	for k, v := range opt.Labels {
		so.FrontendAttrs["label:"+k] = v
	}

	for k, v := range di.ProxyConfig {
		if _, ok := opt.BuildArgs[k]; !ok {
			so.FrontendAttrs["build-arg:"+k] = v
		}
	}

	// set platforms
	if len(opt.Platforms) != 0 {
		pp := make([]string, len(opt.Platforms))
		for i, p := range opt.Platforms {
			pp[i] = platforms.Format(p)
		}
		if len(pp) > 1 && !d.Features()[driver.MultiPlatform] {
			return nil, nil, notSupported(d, driver.MultiPlatform)
		}
		so.FrontendAttrs["platform"] = strings.Join(pp, ",")
	}

	// setup networkmode
	switch opt.NetworkMode {
	case "host":
		so.FrontendAttrs["force-network-mode"] = opt.NetworkMode
		so.AllowedEntitlements = append(so.AllowedEntitlements, entitlements.EntitlementNetworkHost)
	case "none":
		so.FrontendAttrs["force-network-mode"] = opt.NetworkMode
	case "", "default":
	default:
		return nil, nil, errors.Errorf("network mode %q not supported by buildkit. You can define a custom network for your builder using the network driver-opt in buildx create.", opt.NetworkMode)
	}

	// setup extrahosts
	extraHosts, err := toBuildkitExtraHosts(opt.ExtraHosts)
	if err != nil {
		return nil, nil, err
	}
	so.FrontendAttrs["add-hosts"] = extraHosts

	// setup shm size
	if opt.ShmSize.Value() > 0 {
		so.FrontendAttrs["shm-size"] = strconv.FormatInt(opt.ShmSize.Value(), 10)
	}

	// setup ulimits
	ulimits, err := toBuildkitUlimits(opt.Ulimits)
	if err != nil {
		return nil, nil, err
	} else if len(ulimits) > 0 {
		so.FrontendAttrs["ulimit"] = ulimits
	}

	return &so, releaseF, nil
}

func Build(ctx context.Context, drivers []DriverInfo, opt map[string]Options, docker DockerAPI, configDir string, w progress.Writer) (resp map[string]*client.SolveResponse, err error) {
	if len(drivers) == 0 {
		return nil, errors.Errorf("driver required for build")
	}

	drivers, err = filterAvailableDrivers(drivers)
	if err != nil {
		return nil, errors.Wrapf(err, "no valid drivers found")
	}

	var noMobyDriver driver.Driver
	for _, d := range drivers {
		if !d.Driver.IsMobyDriver() {
			noMobyDriver = d.Driver
			break
		}
	}

	if noMobyDriver != nil && !noDefaultLoad() {
		for _, opt := range opt {
			if len(opt.Exports) == 0 {
				logrus.Warnf("No output specified for %s driver. Build result will only remain in the build cache. To push result image into registry use --push or to load image into docker use --load", noMobyDriver.Factory().Name())
				break
			}
		}
	}

	m, clients, err := resolveDrivers(ctx, drivers, opt, w)
	if err != nil {
		return nil, err
	}

	defers := make([]func(), 0, 2)
	defer func() {
		if err != nil {
			for _, f := range defers {
				f()
			}
		}
	}()

	eg, ctx := errgroup.WithContext(ctx)

	for k, opt := range opt {
		multiDriver := len(m[k]) > 1
		hasMobyDriver := false
		for i, dp := range m[k] {
			di := drivers[dp.driverIndex]
			if di.Driver.IsMobyDriver() {
				hasMobyDriver = true
			}
			opt.Platforms = dp.platforms
			so, release, err := toSolveOpt(ctx, di, multiDriver, opt, dp.bopts, configDir, w, func(name string) (io.WriteCloser, func(), error) {
				return newDockerLoader(ctx, docker, name, w)
			})
			if err != nil {
				return nil, err
			}
			defers = append(defers, release)
			m[k][i].so = so
		}
		for _, at := range opt.Session {
			if s, ok := at.(interface {
				SetLogger(progresswriter.Logger)
			}); ok {
				s.SetLogger(func(s *client.SolveStatus) {
					w.Write(s)
				})
			}
		}

		// validate for multi-node push
		if hasMobyDriver && multiDriver {
			for _, dp := range m[k] {
				for _, e := range dp.so.Exports {
					if e.Type == "moby" {
						if ok, _ := strconv.ParseBool(e.Attrs["push"]); ok {
							return nil, errors.Errorf("multi-node push can't currently be performed with the docker driver, please switch to a different driver")
						}
					}
				}
			}
		}
	}

	// validate that all links between targets use same drivers
	for name := range opt {
		dps := m[name]
		for _, dp := range dps {
			for k, v := range dp.so.FrontendAttrs {
				if strings.HasPrefix(k, "context:") && strings.HasPrefix(v, "target:") {
					k2 := strings.TrimPrefix(v, "target:")
					dps2, ok := m[k2]
					if !ok {
						return nil, errors.Errorf("failed to find target %s for context %s", k2, strings.TrimPrefix(k, "context:")) // should be validated before already
					}
					var found bool
					for _, dp2 := range dps2 {
						if dp2.driverIndex == dp.driverIndex {
							found = true
							break
						}
					}
					if !found {
						return nil, errors.Errorf("failed to use %s as context %s for %s because targets build with different drivers", k2, strings.TrimPrefix(k, "context:"), name)
					}
				}
			}
		}
	}

	resp = map[string]*client.SolveResponse{}
	var respMu sync.Mutex
	results := waitmap.New()

	multiTarget := len(opt) > 1

	for k, opt := range opt {
		err := func(k string) error {
			opt := opt
			dps := m[k]
			multiDriver := len(m[k]) > 1

			var span trace.Span
			ctx := ctx
			if multiTarget {
				span, ctx = tracing.StartSpan(ctx, k)
			}

			res := make([]*client.SolveResponse, len(dps))
			wg := &sync.WaitGroup{}
			wg.Add(len(dps))

			var pushNames string
			var insecurePush bool

			eg.Go(func() (err error) {
				defer func() {
					if span != nil {
						tracing.FinishWithError(span, err)
					}
				}()
				pw := progress.WithPrefix(w, "default", false)
				wg.Wait()
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				respMu.Lock()
				resp[k] = res[0]
				respMu.Unlock()
				if len(res) == 1 {
					dgst := res[0].ExporterResponse[exptypes.ExporterImageDigestKey]
					if v, ok := res[0].ExporterResponse[exptypes.ExporterImageConfigDigestKey]; ok {
						dgst = v
					}
					if opt.ImageIDFile != "" {
						return ioutil.WriteFile(opt.ImageIDFile, []byte(dgst), 0644)
					}
					return nil
				}

				if pushNames != "" {
					progress.Write(pw, fmt.Sprintf("merging manifest list %s", pushNames), func() error {
						descs := make([]specs.Descriptor, 0, len(res))

						for _, r := range res {
							s, ok := r.ExporterResponse[exptypes.ExporterImageDigestKey]
							if ok {
								descs = append(descs, specs.Descriptor{
									Digest:    digest.Digest(s),
									MediaType: images.MediaTypeDockerSchema2ManifestList,
									Size:      -1,
								})
							}
						}
						if len(descs) > 0 {
							var imageopt imagetools.Opt
							for _, dp := range dps {
								imageopt = drivers[dp.driverIndex].ImageOpt
								break
							}
							names := strings.Split(pushNames, ",")

							if insecurePush {
								insecureTrue := true
								httpTrue := true
								nn, err := reference.ParseNormalizedNamed(names[0])
								if err != nil {
									return err
								}
								imageopt.RegistryConfig = map[string]resolver.RegistryConfig{
									reference.Domain(nn): {
										Insecure:  &insecureTrue,
										PlainHTTP: &httpTrue,
									},
								}
							}

							itpull := imagetools.New(imageopt)

							dt, desc, err := itpull.Combine(ctx, names[0], descs)
							if err != nil {
								return err
							}
							if opt.ImageIDFile != "" {
								if err := ioutil.WriteFile(opt.ImageIDFile, []byte(desc.Digest), 0644); err != nil {
									return err
								}
							}

							itpush := imagetools.New(imageopt)

							for _, n := range names {
								nn, err := reference.ParseNormalizedNamed(n)
								if err != nil {
									return err
								}
								if err := itpush.Push(ctx, nn, desc, dt); err != nil {
									return err
								}
							}

							respMu.Lock()
							resp[k] = &client.SolveResponse{
								ExporterResponse: map[string]string{
									"containerimage.digest": desc.Digest.String(),
								},
							}
							respMu.Unlock()
						}
						return nil
					})
				}
				return nil
			})

			for i, dp := range dps {
				so := *dp.so
				if multiDriver {
					for i, e := range so.Exports {
						switch e.Type {
						case "oci", "tar":
							return errors.Errorf("%s for multi-node builds currently not supported", e.Type)
						case "image":
							if pushNames == "" && e.Attrs["push"] != "" {
								if ok, _ := strconv.ParseBool(e.Attrs["push"]); ok {
									pushNames = e.Attrs["name"]
									if pushNames == "" {
										return errors.Errorf("tag is needed when pushing to registry")
									}
									names, err := toRepoOnly(e.Attrs["name"])
									if err != nil {
										return err
									}
									if ok, _ := strconv.ParseBool(e.Attrs["registry.insecure"]); ok {
										insecurePush = true
									}
									e.Attrs["name"] = names
									e.Attrs["push-by-digest"] = "true"
									so.Exports[i].Attrs = e.Attrs
								}
							}
						}
					}
				}

				func(i int, dp driverPair, so client.SolveOpt) {
					pw := progress.WithPrefix(w, k, multiTarget)

					c := clients[dp.driverIndex]
					eg.Go(func() error {
						pw = progress.ResetTime(pw)
						defer wg.Done()

						if err := waitContextDeps(ctx, dp.driverIndex, results, &so); err != nil {
							return err
						}

						frontendInputs := make(map[string]*pb.Definition)
						for key, st := range so.FrontendInputs {
							def, err := st.Marshal(ctx)
							if err != nil {
								return err
							}
							frontendInputs[key] = def.ToPB()
						}

						req := gateway.SolveRequest{
							Frontend:       so.Frontend,
							FrontendOpt:    so.FrontendAttrs,
							FrontendInputs: frontendInputs,
						}
						so.Frontend = ""
						so.FrontendAttrs = nil
						so.FrontendInputs = nil

						ch, done := progress.NewChannel(pw)
						defer func() { <-done }()

						rr, err := c.Build(ctx, so, "buildx", func(ctx context.Context, c gateway.Client) (*gateway.Result, error) {
							res, err := c.Solve(ctx, req)
							if err != nil {
								return nil, err
							}
							results.Set(resultKey(dp.driverIndex, k), res)
							return res, nil
						}, ch)
						if err != nil {
							return err
						}
						res[i] = rr

						d := drivers[dp.driverIndex].Driver
						if d.IsMobyDriver() {
							for _, e := range so.Exports {
								if e.Type == "moby" && e.Attrs["push"] != "" {
									if ok, _ := strconv.ParseBool(e.Attrs["push"]); ok {
										pushNames = e.Attrs["name"]
										if pushNames == "" {
											return errors.Errorf("tag is needed when pushing to registry")
										}
										pw := progress.ResetTime(pw)
										pushList := strings.Split(pushNames, ",")
										for _, name := range pushList {
											if err := progress.Wrap(fmt.Sprintf("pushing %s with docker", name), pw.Write, func(l progress.SubLogger) error {
												return pushWithMoby(ctx, d, name, l)
											}); err != nil {
												return err
											}
										}
										remoteDigest, err := remoteDigestWithMoby(ctx, d, pushList[0])
										if err == nil && remoteDigest != "" {
											// old daemons might not have containerimage.config.digest set
											// in response so use containerimage.digest value for it if available
											if _, ok := rr.ExporterResponse[exptypes.ExporterImageConfigDigestKey]; !ok {
												if v, ok := rr.ExporterResponse[exptypes.ExporterImageDigestKey]; ok {
													rr.ExporterResponse[exptypes.ExporterImageConfigDigestKey] = v
												}
											}
											rr.ExporterResponse[exptypes.ExporterImageDigestKey] = remoteDigest
										} else if err != nil {
											return err
										}
									}
								}
							}
						}
						return nil
					})

				}(i, dp, so)
			}

			return nil
		}(k)
		if err != nil {
			return nil, err
		}
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return resp, nil
}

func pushWithMoby(ctx context.Context, d driver.Driver, name string, l progress.SubLogger) error {
	api := d.Config().DockerAPI
	if api == nil {
		return errors.Errorf("invalid empty Docker API reference") // should never happen
	}
	creds, err := imagetools.RegistryAuthForRef(name, d.Config().Auth)
	if err != nil {
		return err
	}

	rc, err := api.ImagePush(ctx, name, types.ImagePushOptions{
		RegistryAuth: creds,
	})
	if err != nil {
		return err
	}

	started := map[string]*client.VertexStatus{}

	defer func() {
		for _, st := range started {
			if st.Completed == nil {
				now := time.Now()
				st.Completed = &now
				l.SetStatus(st)
			}
		}
	}()

	dec := json.NewDecoder(rc)
	var parsedError error
	for {
		var jm jsonmessage.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if parsedError != nil {
				return parsedError
			}
			if err == io.EOF {
				break
			}
			return err
		}
		if jm.ID != "" {
			id := "pushing layer " + jm.ID
			st, ok := started[id]
			if !ok {
				if jm.Progress != nil || jm.Status == "Pushed" {
					now := time.Now()
					st = &client.VertexStatus{
						ID:      id,
						Started: &now,
					}
					started[id] = st
				} else {
					continue
				}
			}
			st.Timestamp = time.Now()
			if jm.Progress != nil {
				st.Current = jm.Progress.Current
				st.Total = jm.Progress.Total
			}
			if jm.Error != nil {
				now := time.Now()
				st.Completed = &now
			}
			if jm.Status == "Pushed" {
				now := time.Now()
				st.Completed = &now
				st.Current = st.Total
			}
			l.SetStatus(st)
		}
		if jm.Error != nil {
			parsedError = jm.Error
		}
	}
	return nil
}

func remoteDigestWithMoby(ctx context.Context, d driver.Driver, name string) (string, error) {
	api := d.Config().DockerAPI
	if api == nil {
		return "", errors.Errorf("invalid empty Docker API reference") // should never happen
	}
	creds, err := imagetools.RegistryAuthForRef(name, d.Config().Auth)
	if err != nil {
		return "", err
	}
	image, _, err := api.ImageInspectWithRaw(ctx, name)
	if err != nil {
		return "", err
	}
	if len(image.RepoDigests) == 0 {
		return "", nil
	}
	remoteImage, err := api.DistributionInspect(ctx, name, creds)
	if err != nil {
		return "", err
	}
	return remoteImage.Descriptor.Digest.String(), nil
}

func createTempDockerfile(r io.Reader) (string, error) {
	dir, err := ioutil.TempDir("", "dockerfile")
	if err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return dir, err
}

func LoadInputs(ctx context.Context, d driver.Driver, inp Inputs, pw progress.Writer, target *client.SolveOpt) (func(), error) {
	if inp.ContextPath == "" {
		return nil, errors.New("please specify build context (e.g. \".\" for the current directory)")
	}

	// TODO: handle stdin, symlinks, remote contexts, check files exist

	var (
		err              error
		dockerfileReader io.Reader
		dockerfileDir    string
		dockerfileName   = inp.DockerfilePath
		toRemove         []string
	)

	switch {
	case inp.ContextState != nil:
		if target.FrontendInputs == nil {
			target.FrontendInputs = make(map[string]llb.State)
		}
		target.FrontendInputs["context"] = *inp.ContextState
		target.FrontendInputs["dockerfile"] = *inp.ContextState
	case inp.ContextPath == "-":
		if inp.DockerfilePath == "-" {
			return nil, errStdinConflict
		}

		buf := bufio.NewReader(inp.InStream)
		magic, err := buf.Peek(archiveHeaderSize * 2)
		if err != nil && err != io.EOF {
			return nil, errors.Wrap(err, "failed to peek context header from STDIN")
		}
		if !(err == io.EOF && len(magic) == 0) {
			if isArchive(magic) {
				// stdin is context
				up := uploadprovider.New()
				target.FrontendAttrs["context"] = up.Add(buf)
				target.Session = append(target.Session, up)
			} else {
				if inp.DockerfilePath != "" {
					return nil, errDockerfileConflict
				}
				// stdin is dockerfile
				dockerfileReader = buf
				inp.ContextPath, _ = ioutil.TempDir("", "empty-dir")
				toRemove = append(toRemove, inp.ContextPath)
				target.LocalDirs["context"] = inp.ContextPath
			}
		}

	case isLocalDir(inp.ContextPath):
		target.LocalDirs["context"] = inp.ContextPath
		switch inp.DockerfilePath {
		case "-":
			dockerfileReader = inp.InStream
		case "":
			dockerfileDir = inp.ContextPath
		default:
			dockerfileDir = filepath.Dir(inp.DockerfilePath)
			dockerfileName = filepath.Base(inp.DockerfilePath)
		}

	case urlutil.IsGitURL(inp.ContextPath), urlutil.IsURL(inp.ContextPath):
		if inp.DockerfilePath == "-" {
			return nil, errors.Errorf("Dockerfile from stdin is not supported with remote contexts")
		}
		target.FrontendAttrs["context"] = inp.ContextPath
	default:
		return nil, errors.Errorf("unable to prepare context: path %q not found", inp.ContextPath)
	}

	if inp.DockerfileInline != "" {
		dockerfileReader = strings.NewReader(inp.DockerfileInline)
	}

	if dockerfileReader != nil {
		dockerfileDir, err = createTempDockerfile(dockerfileReader)
		if err != nil {
			return nil, err
		}
		toRemove = append(toRemove, dockerfileDir)
		dockerfileName = "Dockerfile"
		target.FrontendAttrs["dockerfilekey"] = "dockerfile"
	}
	if urlutil.IsURL(inp.DockerfilePath) {
		dockerfileDir, err = createTempDockerfileFromURL(ctx, d, inp.DockerfilePath, pw)
		if err != nil {
			return nil, err
		}
		toRemove = append(toRemove, dockerfileDir)
		dockerfileName = "Dockerfile"
		target.FrontendAttrs["dockerfilekey"] = "dockerfile"
		delete(target.FrontendInputs, "dockerfile")
	}

	if dockerfileName == "" {
		dockerfileName = "Dockerfile"
	}

	if dockerfileDir != "" {
		target.LocalDirs["dockerfile"] = dockerfileDir
		dockerfileName = handleLowercaseDockerfile(dockerfileDir, dockerfileName)
	}

	target.FrontendAttrs["filename"] = dockerfileName

	for k, v := range inp.NamedContexts {
		target.FrontendAttrs["frontend.caps"] = "moby.buildkit.frontend.contexts+forward"
		if v.State != nil {
			target.FrontendAttrs["context:"+k] = "input:" + k
			if target.FrontendInputs == nil {
				target.FrontendInputs = make(map[string]llb.State)
			}
			target.FrontendInputs[k] = *v.State
			continue
		}

		if urlutil.IsGitURL(v.Path) || urlutil.IsURL(v.Path) || strings.HasPrefix(v.Path, "docker-image://") || strings.HasPrefix(v.Path, "target:") {
			target.FrontendAttrs["context:"+k] = v.Path
			continue
		}
		st, err := os.Stat(v.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get build context %v", k)
		}
		if !st.IsDir() {
			return nil, errors.Wrapf(syscall.ENOTDIR, "failed to get build context path %v", v)
		}
		localName := k
		if k == "context" || k == "dockerfile" {
			localName = "_" + k // underscore to avoid collisions
		}
		target.LocalDirs[localName] = v.Path
		target.FrontendAttrs["context:"+k] = "local:" + localName
	}

	release := func() {
		for _, dir := range toRemove {
			os.RemoveAll(dir)
		}
	}
	return release, nil
}

func resultKey(index int, name string) string {
	return fmt.Sprintf("%d-%s", index, name)
}

func waitContextDeps(ctx context.Context, index int, results *waitmap.Map, so *client.SolveOpt) error {
	m := map[string]string{}
	for k, v := range so.FrontendAttrs {
		if strings.HasPrefix(k, "context:") && strings.HasPrefix(v, "target:") {
			target := resultKey(index, strings.TrimPrefix(v, "target:"))
			m[target] = k
		}
	}
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	res, err := results.Get(ctx, keys...)
	if err != nil {
		return err
	}

	for k, v := range m {
		r, ok := res[k]
		if !ok {
			continue
		}
		rr, ok := r.(*gateway.Result)
		if !ok {
			return errors.Errorf("invalid result type %T", rr)
		}
		if so.FrontendAttrs == nil {
			so.FrontendAttrs = map[string]string{}
		}
		if so.FrontendInputs == nil {
			so.FrontendInputs = map[string]llb.State{}
		}
		if len(rr.Refs) > 0 {
			for platform, r := range rr.Refs {
				st, err := r.ToState()
				if err != nil {
					return err
				}
				so.FrontendInputs[k+"::"+platform] = st
				so.FrontendAttrs[v+"::"+platform] = "input:" + k + "::" + platform
				metadata := make(map[string][]byte)
				if dt, ok := rr.Metadata[exptypes.ExporterImageConfigKey+"/"+platform]; ok {
					metadata[exptypes.ExporterImageConfigKey] = dt
				}
				if dt, ok := rr.Metadata[exptypes.ExporterBuildInfo+"/"+platform]; ok {
					metadata[exptypes.ExporterBuildInfo] = dt
				}
				if len(metadata) > 0 {
					dt, err := json.Marshal(metadata)
					if err != nil {
						return err
					}
					so.FrontendAttrs["input-metadata:"+k+"::"+platform] = string(dt)
				}
			}
			delete(so.FrontendAttrs, v)
		}
		if rr.Ref != nil {
			st, err := rr.Ref.ToState()
			if err != nil {
				return err
			}
			so.FrontendInputs[k] = st
			so.FrontendAttrs[v] = "input:" + k
			metadata := make(map[string][]byte)
			if dt, ok := rr.Metadata[exptypes.ExporterImageConfigKey]; ok {
				metadata[exptypes.ExporterImageConfigKey] = dt
			}
			if dt, ok := rr.Metadata[exptypes.ExporterBuildInfo]; ok {
				metadata[exptypes.ExporterBuildInfo] = dt
			}
			if len(metadata) > 0 {
				dt, err := json.Marshal(metadata)
				if err != nil {
					return err
				}
				so.FrontendAttrs["input-metadata:"+k] = string(dt)
			}
		}
	}
	return nil
}

func notSupported(d driver.Driver, f driver.Feature) error {
	return errors.Errorf("%s feature is currently not supported for %s driver. Please switch to a different driver (eg. \"docker buildx create --use\")", f, d.Factory().Name())
}

type dockerLoadCallback func(name string) (io.WriteCloser, func(), error)

func newDockerLoader(ctx context.Context, d DockerAPI, name string, status progress.Writer) (io.WriteCloser, func(), error) {
	c, err := d.DockerAPI(name)
	if err != nil {
		return nil, nil, err
	}

	pr, pw := io.Pipe()
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(ctx)
	var w *waitingWriter
	w = &waitingWriter{
		PipeWriter: pw,
		f: func() {
			resp, err := c.ImageLoad(ctx, pr, false)
			defer close(done)
			if err != nil {
				pr.CloseWithError(err)
				w.mu.Lock()
				w.err = err
				w.mu.Unlock()
				return
			}
			prog := progress.WithPrefix(status, "", false)
			progress.FromReader(prog, "importing to docker", resp.Body)
		},
		done:   done,
		cancel: cancel,
	}
	return w, func() {
		pr.Close()
	}, nil
}

func noDefaultLoad() bool {
	v, ok := os.LookupEnv("BUILDX_NO_DEFAULT_LOAD")
	if !ok {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		logrus.Warnf("invalid non-bool value for BUILDX_NO_DEFAULT_LOAD: %s", v)
	}
	return b
}

type waitingWriter struct {
	*io.PipeWriter
	f      func()
	once   sync.Once
	mu     sync.Mutex
	err    error
	done   chan struct{}
	cancel func()
}

func (w *waitingWriter) Write(dt []byte) (int, error) {
	w.once.Do(func() {
		go w.f()
	})
	return w.PipeWriter.Write(dt)
}

func (w *waitingWriter) Close() error {
	err := w.PipeWriter.Close()
	<-w.done
	if err == nil {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.err
	}
	return err
}

// handle https://github.com/moby/moby/pull/10858
func handleLowercaseDockerfile(dir, p string) string {
	if filepath.Base(p) != "Dockerfile" {
		return p
	}

	f, err := os.Open(filepath.Dir(filepath.Join(dir, p)))
	if err != nil {
		return p
	}

	names, err := f.Readdirnames(-1)
	if err != nil {
		return p
	}

	foundLowerCase := false
	for _, n := range names {
		if n == "Dockerfile" {
			return p
		}
		if n == "dockerfile" {
			foundLowerCase = true
		}
	}
	if foundLowerCase {
		return filepath.Join(filepath.Dir(p), "dockerfile")
	}
	return p
}

func wrapWriteCloser(wc io.WriteCloser) func(map[string]string) (io.WriteCloser, error) {
	return func(map[string]string) (io.WriteCloser, error) {
		return wc, nil
	}
}

var nodeIdentifierMu sync.Mutex

func tryNodeIdentifier(configDir string) (out string) {
	nodeIdentifierMu.Lock()
	defer nodeIdentifierMu.Unlock()
	sessionFile := filepath.Join(configDir, ".buildNodeID")
	if _, err := os.Lstat(sessionFile); err != nil {
		if os.IsNotExist(err) { // create a new file with stored randomness
			b := make([]byte, 8)
			if _, err := rand.Read(b); err != nil {
				return out
			}
			if err := ioutil.WriteFile(sessionFile, []byte(hex.EncodeToString(b)), 0600); err != nil {
				return out
			}
		}
	}

	dt, err := ioutil.ReadFile(sessionFile)
	if err == nil {
		return string(dt)
	}
	return
}
