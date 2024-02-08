package build

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/image"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/pod"
	"github.com/slimtoolkit/slim/pkg/app/master/kubernetes"
	"github.com/slimtoolkit/slim/pkg/app/master/probe/http"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/resource"
)

type kubeHandler struct {
	*app.ExecutionContext
	ctx    context.Context
	report *report.BuildCommand
	logger *log.Entry

	dockerClient *dockerapi.Client
	kubeClient   *kubernetes.Client
	kubectl      kubernetes.Kubectl

	finder *kubernetes.WorkloadFinder
}

func newKubeHandler(
	xc *app.ExecutionContext,
	ctx context.Context,
	cmdReport *report.BuildCommand,
	logger *log.Entry,
	dockerClient *dockerapi.Client,
	kubeClient *kubernetes.Client,
	kubectl kubernetes.Kubectl,
	finder *kubernetes.WorkloadFinder,
) *kubeHandler {
	return &kubeHandler{
		ctx:              ctx,
		ExecutionContext: xc,
		report:           cmdReport,
		logger:           logger,

		dockerClient: dockerClient,
		kubeClient:   kubeClient,
		kubectl:      kubectl,

		finder: finder,
	}
}

type kubeHandleOptions struct {
	DoPull                    bool
	DoShowPullLogs            bool
	DoShowBuildLogs           bool
	DoShowContainerLogs       bool
	DoEnableMondel            bool
	DoDeleteFatImage          bool
	DoRmFileArtifacts         bool
	RtaOnbuildBaseImage       bool
	RtaSourcePT               bool
	DockerConfigPath          string
	RegistryAccount           string
	RegistrySecret            string
	KeepPerms                 bool
	PathPerms                 map[string]*fsutil.AccessInfo
	ArchiveState              string
	StatePath                 string
	CopyMetaArtifactsLocation string
	Debug                     bool
	LogLevel                  string
	LogFormat                 string
	SensorIPCEndpoint         string
	CBOpts                    *config.ContainerBuildOptions

	CustomImageTag string
	AdditionalTags []string

	PortBindings          map[dockerapi.Port][]dockerapi.PortBinding
	DoPublishExposedPorts bool

	httpProbeOpts    config.HTTPProbeOptions
	continueAfter    *config.ContinueAfter
	execCmd          string
	imageBuildEngine string
	imageBuildArch   string
}

func (h *kubeHandler) Handle(
	target config.KubernetesTarget,
	targetOverride config.KubernetesTargetOverride,
	manifests *kubernetes.Manifests,
	opts kubeHandleOptions,
) {
	// 1. Pre-process the workload
	//    - find the running workload in the cluster
	//    - ...or in the supplied manifest
	workload := h.findWorkloadOrFail(target)

	if manifests != nil {
		h.applyManifestsOrFail(manifests, workload)
	}

	// 2. Inspect the workload's original image.
	if targetOverride.Image != "" {
		workload.TargetContainer().Image = command.UpdateImageRef(
			h.logger,
			workload.TargetContainer().Image,
			targetOverride.Image)
	}

	imageInspector, _, statePath, stateKey := inspectFatImage(
		h.ExecutionContext,
		workload.TargetContainer().Image,
		opts.DoPull,
		opts.DoShowPullLogs,
		opts.RtaOnbuildBaseImage,
		opts.DockerConfigPath,
		opts.RegistryAccount,
		opts.RegistrySecret,
		opts.StatePath,
		h.dockerClient,
		h.logger,
		h.report)
	workload.TargetContainer().Image = imageInspector.ImageRef

	// 3. Patch and run the workload
	//    - patch: add the init container, the volume, replace the entrypoint
	//    - copy sensor to the volume via the init container
	//    - terminate (successfully) the init container
	//    - send StartCommand to the sensor
	podInspector, err := pod.NewInspector(
		h.ctx,
		h.ExecutionContext,
		h.logger,
		workload,
		h.kubectl,
		h.kubeClient,
		imageInspector,
		opts.KeepPerms,
		opts.PathPerms,
		opts.Debug,
		opts.LogLevel,
		opts.LogFormat,
		opts.RtaSourcePT,
		statePath,
		nil,
		opts.SensorIPCEndpoint,
		opts.PortBindings,
		opts.DoPublishExposedPorts,
	)
	h.FailOn(err)

	h.AddCleanupHandler(func() {
		podInspector.FinishMonitoring()
		podInspector.ShutdownPod(manifests == nil)
	})

	h.logger.Info("starting instrumented 'fat' workload...")
	err = podInspector.RunPod()
	if err != nil && opts.DoShowContainerLogs {
		podInspector.ShowPodLogs()
	}
	h.FailOn(err)

	h.Out.Info("pod",
		ovars{
			"name":             podInspector.PodName(),
			"target.port.list": podInspector.PodPortList(),
			"target.port.info": podInspector.PodPortsInfo(),
			"message":          "YOU CAN USE THESE PORTS TO INTERACT WITH THE POD",
		})

	// 4. Monitor the workload.
	h.logger.Info("watching pod monitor...")
	h.monitorPod(opts, podInspector)

	// 5. Copy the artifact from the workload.
	h.Out.State("pod.inspection.finishing")

	podInspector.FinishMonitoring()

	// 6. Shut down the workload.
	h.logger.Info("shutting down 'fat' pod...")
	podInspector.ShutdownPod(manifests == nil)

	if manifests != nil {
		errutil.WarnOn(manifests.Delete(h.ctx))
	}

	// 7. Build the slim image & create AppArmor and seccomp profiles
	h.processCollectedDataOrFail(podInspector, imageInspector)

	minifiedImageName := buildOutputImage(
		h.ExecutionContext,
		opts.CustomImageTag,
		opts.AdditionalTags,
		opts.CBOpts,
		nil, // TODO: overrrides
		nil, // TODO: imageOverrideSelectors,
		nil, // TODO: instructions,
		opts.DoDeleteFatImage,
		opts.DoShowBuildLogs,
		imageInspector,
		h.dockerClient,
		h.logger,
		h.report,
		opts.imageBuildEngine,
		opts.imageBuildArch)

	finishCommand(
		h.ExecutionContext,
		minifiedImageName,
		opts.CopyMetaArtifactsLocation,
		opts.DoRmFileArtifacts,
		opts.ArchiveState,
		stateKey,
		imageInspector,
		h.dockerClient,
		h.logger,
		h.report,
		opts.imageBuildEngine)
}

func (h *kubeHandler) findWorkloadOrFail(target config.KubernetesTarget) *kubernetes.Workload {
	workload, err := h.finder.Find(target)
	h.FailOn(err)

	if workload == nil {
		h.Out.Info("kubernetes.workload.error",
			ovars{
				"status": "workload.not.found",
				"target": target.Workload,
			})

		exitCode := command.ECTBuild | ecbKubernetesNoWorkload
		h.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		h.Exit(exitCode)
	}

	if workload.TargetContainer() == nil {
		h.Out.Info("kubernetes.workload.error",
			ovars{
				"status":           "container.not.found",
				"target.container": target.Container,
			})

		exitCode := command.ECTBuild | ecbKubernetesNoWorkloadContainer
		h.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		h.Exit(exitCode)
	}

	h.Out.Info("kubernetes.workload",
		ovars{
			"namespace": workload.Namespace(),
			"name":      workload.Name(),
			"template":  asJSON(workload.Template()),
			"target":    asJSON(workload.TargetContainer()),
		})

	return workload
}

func (h *kubeHandler) applyManifestsOrFail(
	manifests *kubernetes.Manifests,
	workload *kubernetes.Workload,
) {
	h.AddCleanupHandler(func() {
		errutil.WarnOn(manifests.Delete(h.ctx))
	})

	// PodInspector will be applying the workload manifest separately.
	err := manifests.Apply(h.ctx, func(info *resource.Info) bool {
		return workload.Info().Mapping.GroupVersionKind.GroupKind() != info.Mapping.GroupVersionKind.GroupKind() ||
			workload.Name() != info.Name || workload.Namespace() != info.Namespace
	})
	h.FailOn(err)
}

func (h *kubeHandler) monitorPod(
	opts kubeHandleOptions,
	podInspector *pod.Inspector,
) {
	var probe *http.CustomProbe
	if opts.httpProbeOpts.Do {
		var err error
		probe, err = http.NewPodProbe(h.ExecutionContext, podInspector, opts.httpProbeOpts, true)
		h.FailOn(err)

		if len(probe.Ports()) == 0 {
			h.Out.State("http.probe.error",
				ovars{
					"error":   "NO EXPOSED PORTS",
					"message": "make sure target container spec has `ports` field filled or disable HTTP probing with --http-probe=false if your Kubernetes workload doesn't expose any network services",
				})

			exitCode := command.ECTBuild | ecbImageBuildError
			h.Out.State("exited", ovars{"exit.code": exitCode})

			h.report.Error = "no.exposed.ports"
			h.Exit(exitCode)
		}

		probe.Start()
		opts.continueAfter.ContinueChan = probe.DoneChan()
	}

	continueAfterMsg := "provide the expected input to allow the container inspector to continue its execution"
	if opts.continueAfter.Mode == config.CAMTimeout {
		continueAfterMsg = "no input required, execution will resume after the timeout"
	}

	if hasContinueAfterMode(opts.continueAfter.Mode, config.CAMProbe) {
		continueAfterMsg = "no input required, execution will resume when HTTP probing is completed"
	}

	h.Out.Info("continue.after",
		ovars{
			"mode":    opts.continueAfter.Mode,
			"message": continueAfterMsg,
		})

	modes := strings.Split(opts.continueAfter.Mode, "&")
	for _, mode := range modes {
		switch mode {
		case config.CAMEnter:
			h.Out.Prompt("USER INPUT REQUIRED, PRESS <ENTER> WHEN YOU ARE DONE USING THE KUBERNETES WORKLOAD")
			creader := bufio.NewReader(os.Stdin)
			_, _, _ = creader.ReadLine()

		case config.CAMExec:
			// Use execCmd
			h.Out.Info("continue.after", ovars{"mode": config.CAMExec, "shell": opts.execCmd})

			out, err := podInspector.Exec("sh", "-c", opts.execCmd)
			errutil.WarnOn(err)

			h.Out.Info("continue.after", ovars{"mode": config.CAMExec, "output": string(out)})

		case config.CAMSignal:
			h.Out.Prompt("send SIGUSR1 when you are done using the container")
			<-opts.continueAfter.ContinueChan
			h.Out.Info("event", ovars{"message": "got SIGUSR1"})

		case config.CAMTimeout:
			h.Out.Prompt(fmt.Sprintf("waiting for the target container (%v seconds)", int(opts.continueAfter.Timeout)))
			<-time.After(time.Second * opts.continueAfter.Timeout)
			h.Out.Info("event", ovars{"message": "done waiting for the target container"})

		case config.CAMProbe:
			h.Out.Prompt("waiting for the HTTP probe to finish")
			<-opts.continueAfter.ContinueChan
			h.Out.Info("event", ovars{"message": "HTTP probe is done"})

			if probe.CallCount > 0 && probe.OkCount == 0 && opts.httpProbeOpts.ExitOnFailure {
				h.Out.Error("probe.error", "no.successful.calls")

				podInspector.ShowPodLogs()
				h.Out.State("exited", ovars{"exit.code": -1})
				h.Exit(-1)
			}

		default:
			errutil.Fail("unknown continue-after mode")
		}
	}
}

func (h *kubeHandler) processCollectedDataOrFail(podInspector *pod.Inspector, imageInspector *image.Inspector) {
	if !podInspector.HasCollectedData() {
		imageInspector.ShowFatImageDockerInstructions()
		h.Out.Info("results",
			ovars{
				"status":   "no data collected (no minified image generated)",
				"version":  v.Current(),
				"location": fsutil.ExeDir(),
			})

		exitCode := command.ECTBuild | ecbImageBuildError
		h.Out.State("exited",
			ovars{
				"exit.code": exitCode,
			})

		h.report.Error = "no.data.collected"
		h.Exit(exitCode)
	}

	h.logger.Info("processing instrumented 'fat' container info...")
	h.FailOn(podInspector.ProcessCollectedData())
}

func asJSON(val interface{}) string {
	bytes, err := json.Marshal(val)
	if err != nil {
		panic("json.Marshal failed: " + err.Error())
	}
	return string(bytes)
}
