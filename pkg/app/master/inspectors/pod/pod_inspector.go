package pod

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/image"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/ipc"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/sensor"
	"github.com/slimtoolkit/slim/pkg/app/master/kubernetes"
	"github.com/slimtoolkit/slim/pkg/app/master/security/apparmor"
	"github.com/slimtoolkit/slim/pkg/app/master/security/seccomp"
	"github.com/slimtoolkit/slim/pkg/ipc/channel"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type ovars = app.OutVars

// TODO: unify with similar constants in container_inspector.go
const (
	sensorVolumeName      = "slim-sensor"
	sensorVolumeMountPath = "/opt/_slim/bin"
	sensorBinFileAbs      = sensorVolumeMountPath + "/" + sensor.LocalBinFile
	sensorLoaderContainer = "slim-sensor-loader"

	artifactsVolumeName = "slim-artifacts"

	targetPodLabelName = "dockersl.im/target-pod"
	targetPodLabelPat  = "slimk_%v_%v"
)

type portInfo struct {
	// Ports found to be available for probing.
	availablePorts map[dockerapi.Port]dockerapi.PortBinding
	podPortsInfo   []string
	podPortList    []string

	// Needed only to populate the struct during the RunPod() call.
	mux sync.Mutex
}

type Inspector struct {
	ctx         context.Context
	ctxCancelFn context.CancelFunc

	xc         *app.ExecutionContext
	logger     *log.Entry
	kubectl    kubernetes.Kubectl
	kubeClient *kubernetes.Client

	workload       *kubernetes.Workload
	imageInspector *image.Inspector

	fatContainerCmd []string
	keepPerms       bool
	pathPerms       map[string]*fsutil.AccessInfo
	// ExcludePatterns       map[string]*fsutil.AccessInfo
	// PreservePaths         map[string]*fsutil.AccessInfo
	// IncludePaths          map[string]*fsutil.AccessInfo
	// IncludeBins           map[string]*fsutil.AccessInfo
	// IncludeExes           map[string]*fsutil.AccessInfo
	// DoIncludeShell        bool
	// DoIncludeCertAll      bool
	// DoIncludeCertBundles  bool
	// DoIncludeCertDirs     bool
	// DoIncludeCertPKAll    bool
	// DoIncludeCertPKDirs   bool
	// DoIncludeNew          bool
	doDebug           bool
	logLevel          string
	logFormat         string
	rtaSourcePT       bool
	sensorIPCEndpoint string
	statePath         string

	portBindings          map[dockerapi.Port][]dockerapi.PortBinding
	doPublishExposedPorts bool
	portInfo              portInfo

	pod             *corev1.Pod
	sensorIPCClient *ipc.Client
}

func NewInspector(
	ctx context.Context,
	xc *app.ExecutionContext,
	logger *log.Entry,
	workload *kubernetes.Workload,
	kubectl kubernetes.Kubectl,
	kubeClient *kubernetes.Client,
	imageInspector *image.Inspector,
	keepPerms bool,
	pathPerms map[string]*fsutil.AccessInfo,
	// TODO: pass these params
	// ExcludePatterns       map[string]*fsutil.AccessInfo
	// PreservePaths         map[string]*fsutil.AccessInfo
	// IncludePaths          map[string]*fsutil.AccessInfo
	// IncludeBins           map[string]*fsutil.AccessInfo
	// IncludeExes           map[string]*fsutil.AccessInfo
	// DoIncludeShell        bool
	// DoIncludeCertAll      bool
	// DoIncludeCertBundles  bool
	// DoIncludeCertDirs     bool
	// DoIncludeCertPKAll    bool
	// DoIncludeCertPKDirs   bool
	// DoIncludeNew          bool
	doDebug bool,
	logLevel string,
	logFormat string,
	rtaSourcePT bool,
	statePath string,
	contOverrides *config.ContainerOverrides,
	sensorIPCEndpoint string,
	portBindings map[dockerapi.Port][]dockerapi.PortBinding,
	doPublishExposedPorts bool,
) (*Inspector, error) {
	ctx, cancelFn := context.WithCancel(ctx)
	return &Inspector{
		ctx:                   ctx,
		ctxCancelFn:           cancelFn,
		xc:                    xc,
		logger:                logger,
		workload:              workload,
		kubectl:               kubectl,
		kubeClient:            kubeClient,
		imageInspector:        imageInspector,
		fatContainerCmd:       fatContainerCmd(workload, imageInspector, contOverrides),
		keepPerms:             keepPerms,
		pathPerms:             pathPerms,
		doDebug:               doDebug,
		logLevel:              logLevel,
		logFormat:             logFormat,
		rtaSourcePT:           rtaSourcePT,
		statePath:             statePath,
		sensorIPCEndpoint:     sensorIPCEndpoint,
		portBindings:          portBindings,
		doPublishExposedPorts: doPublishExposedPorts,
		portInfo: portInfo{
			availablePorts: map[dockerapi.Port]dockerapi.PortBinding{},
		},
	}, nil
}

func (i *Inspector) TargetHost() string {
	// Since at the moment we rely only on `kubectl port-forward`
	return "127.0.0.1"
}

func (i *Inspector) RunPod() error {
	if err := i.prepareWorkload(); err != nil {
		return err
	}

	if err := i.applyWorkload(); err != nil {
		return err
	}

	if err := waitForContainer(i.ctx, i.kubeClient, i.pod.Namespace, i.pod.Name, sensorLoaderContainer, true); err != nil {
		return err
	}

	localSensorPath := sensor.EnsureLocalBinary(i.xc, i.logger, i.statePath, true)
	i.logger.Debugf("RunPod: detected sensor at %q", localSensorPath)

	if err := i.injectSensor(localSensorPath); err != nil {
		return err
	}

	if err := waitForContainer(i.ctx, i.kubeClient, i.pod.Namespace, i.pod.Name, i.workload.TargetContainer().Name, false); err != nil {
		return err
	}

	if err := i.sensorConnect(); err != nil {
		return err
	}

	if err := i.sensorCommandStart(); err != nil {
		return err
	}

	return i.publishPorts()
}

func (i *Inspector) PodName() string {
	return i.pod.Namespace + "/" + i.pod.Name
}

func (i *Inspector) PodPortsInfo() string {
	return strings.Join(i.portInfo.podPortsInfo, ",")
}

func (i *Inspector) PodPortList() string {
	return strings.Join(i.portInfo.podPortList, ",")
}

func (i *Inspector) AvailablePorts() map[dockerapi.Port]dockerapi.PortBinding {
	return i.portInfo.availablePorts
}

func (i *Inspector) FinishMonitoring() {
	if i.sensorIPCClient == nil {
		return
	}

	errutil.WarnOn(i.sensorCommandStop())

	out, err := i.kubectl.CpFrom(
		i.ctx,
		i.pod.Namespace,
		i.pod.Name,
		i.workload.TargetContainer().Name,
		filepath.Join(app.DefaultArtifactsDirPath, report.DefaultContainerReportFileName),
		filepath.Join(i.imageInspector.ArtifactLocation, report.DefaultContainerReportFileName),
	)
	if err != nil {
		errutil.WarnOn(err)
		i.logger.Debugf("RunPod: kubectl cp pod:artifacts:creport -> localArtifactLocation failed with %q: %s", err, string(out))
	}

	out, err = i.kubectl.CpFrom(
		i.ctx,
		i.pod.Namespace,
		i.pod.Name,
		i.workload.TargetContainer().Name,
		filepath.Join(app.DefaultArtifactsDirPath, app.ArtifactFilesDirName),
		filepath.Join(i.imageInspector.ArtifactLocation, app.ArtifactFilesDirName+"/"),
	)
	if err != nil {
		errutil.WarnOn(err)
		i.logger.Debugf("RunPod: kubectl cp pod:artifacts:files -> localArtifactLocation failed with %q: %s", err, string(out))
	}
}

func (i *Inspector) ShowPodLogs() {
	// TODO: Implement me!
	fmt.Println("slim: pod stdout:")
	fmt.Println("slim: pod stderr:")
	fmt.Println("slim: end of pod logs =============")
}

func (i *Inspector) ShutdownPod(resetChanges bool) {
	if i.sensorIPCClient == nil {
		return
	}

	resp, err := i.sensorIPCClient.SendCommand(&command.ShutdownSensor{})
	if err != nil {
		i.logger.Debugf("error sending 'shutdown' => '%v'", err)
	}
	i.logger.Debugf("'shutdown' sensor response => '%v'", resp)

	if resetChanges {
		i.workload.ResetChanges()
		if err := i.kubeClient.CreateOrUpdate(i.ctx, i.workload.Info()); err != nil {
			i.logger.Debugf("error resetting workload changes => '%v'", err)
		}
	} else if i.workload.SetReplicasIfApplicable(0) {
		if err := i.kubeClient.CreateOrUpdate(i.ctx, i.workload.Info()); err != nil {
			i.logger.Debugf("error scaling down the workload => '%v'", err)
		}
	}

	i.sensorDisconnect()
	i.ctxCancelFn()
}

func (i *Inspector) HasCollectedData() bool {
	return fsutil.Exists(filepath.Join(i.imageInspector.ArtifactLocation, report.DefaultContainerReportFileName))
}

func (i *Inspector) ProcessCollectedData() error {
	i.logger.Info("generating AppArmor profile...")
	err := apparmor.GenProfile(i.imageInspector.ArtifactLocation, i.imageInspector.AppArmorProfileName)
	if err != nil {
		return err
	}

	return seccomp.GenProfile(i.imageInspector.ArtifactLocation, i.imageInspector.SeccompProfileName)
}

func (i *Inspector) Exec(cmd string, args ...string) ([]byte, error) {
	return i.kubectl.Exec(
		i.ctx,
		i.pod.Namespace,
		i.pod.Name,
		i.workload.TargetContainer().Name,
		cmd,
		args...,
	)
}

func (i *Inspector) prepareWorkload() error {
	i.workload.Template().Labels[targetPodLabelName] = fmt.Sprintf(
		targetPodLabelPat, os.Getpid(), time.Now().UTC().Format("20060102150405"))

	i.workload.AddEmptyDirVolume(sensorVolumeName)

	i.workload.AddEmptyDirVolume(artifactsVolumeName)

	i.workload.AddInitContainer(corev1.Container{
		Name:  sensorLoaderContainer,
		Image: "alpine",
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf(
				`until [ -f %s ]; do echo "Waiting for sensor to appear..."; sleep 1; done; echo "Sensor found! Exiting..."`,
				sensorBinFileAbs,
			),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: sensorVolumeName, MountPath: sensorVolumeMountPath},
		},
	})

	i.workload.SetReplicasIfApplicable(1)

	targetCont := i.workload.TargetContainer()
	targetCont.VolumeMounts = append(
		targetCont.VolumeMounts,
		corev1.VolumeMount{
			Name:      sensorVolumeName,
			MountPath: sensorVolumeMountPath,
		},
		corev1.VolumeMount{
			Name:      artifactsVolumeName,
			MountPath: app.DefaultArtifactsDirPath,
		},
	)

	targetCont.Command = []string{sensorBinFileAbs}

	if targetCont.SecurityContext == nil {
		targetCont.SecurityContext = &corev1.SecurityContext{}
	}

	targetCont.SecurityContext.Privileged = boolPtr(true)

	// TODO: check if it's already there
	if targetCont.SecurityContext.Capabilities == nil {
		targetCont.SecurityContext.Capabilities = &corev1.Capabilities{}
	}
	targetCont.SecurityContext.Capabilities.Add = append(
		targetCont.SecurityContext.Capabilities.Add,
		"SYS_ADMIN",
	)

	return nil
}

func (i *Inspector) applyWorkload() error {
	if err := i.kubeClient.CreateOrUpdate(i.ctx, i.workload.Info()); err != nil {
		return err
	}
	i.logger.Debugf("RunPod: workload (re)applied. Waiting for pod to start up...")

	pod, err := findPod(i.ctx, i.kubeClient, i.workload.Namespace(), i.workload.Template().Labels[targetPodLabelName])
	if err != nil {
		return err
	}
	i.logger.Debugf("RunPod: found workload pod '%s/%s'. Waiting for sensor-loader container to start up...", pod.Namespace, pod.Name)

	i.xc.Out.Info("pod",
		ovars{
			"status":    "created",
			"namespace": pod.Namespace,
			"name":      pod.Name,
		})

	i.pod = &pod
	return nil
}

func (i *Inspector) publishPorts() error {
	toPublish := map[dockerapi.Port][]dockerapi.PortBinding{}

	if len(i.portBindings) > 0 {
		for contPort, hostPorts := range i.portBindings {
			if contPort.Port() == toStringPort(channel.CmdPort) {
				i.exitIPCPortConflict(hostPorts, "cmd", -126)
			}
			if contPort.Port() == toStringPort(channel.EvtPort) {
				i.exitIPCPortConflict(hostPorts, "evt", -127)
			}
			toPublish[contPort] = hostPorts
		}
	} else {
		for _, portInfo := range i.workload.TargetContainer().Ports {
			if portInfo.Protocol != "" && portInfo.Protocol != corev1.ProtocolTCP {
				continue
			}

			port := toDockerPort(portInfo.ContainerPort)
			toPublish[port] = []dockerapi.PortBinding{}
			if i.doPublishExposedPorts {
				toPublish[port] = append(toPublish[port], dockerapi.PortBinding{
					HostPort: string(port), // same port number
					HostIP:   i.TargetHost(),
				})
			}
		}
	}

	var wg sync.WaitGroup
	for cp, hps := range toPublish {
		wg.Add(1)

		go func(contPort dockerapi.Port, hostPorts []dockerapi.PortBinding) {
			hostIP := "127.0.0.1"
			hostPort := ""
			if len(hostPorts) > 0 {
				hostIP = hostPorts[0].HostIP
				hostPort = hostPorts[0].HostPort
			}

			cmd, hostPort, err := i.kubectl.PortForward(
				i.ctx,
				i.pod.Namespace,
				i.pod.Name,
				hostIP,
				hostPort,
				contPort.Port(),
			)
			if err == nil {
				i.portInfo.mux.Lock()

				i.portInfo.availablePorts[contPort] = dockerapi.PortBinding{HostIP: i.TargetHost(), HostPort: hostPort}
				i.portInfo.podPortsInfo = append(
					i.portInfo.podPortsInfo,
					fmt.Sprintf("%v => %v:%v", contPort, hostIP, hostPort),
				)
				i.portInfo.podPortList = append(i.portInfo.podPortList, hostPort)

				i.portInfo.mux.Unlock()
			}

			wg.Done()

			if err == nil {
				err = cmd.Wait()
			}
			if err != nil {
				i.logger.Warnf("RunPod: kubectl port-forward container port failed. err=%q", err)
			}
		}(cp, hps)
	}

	wg.Wait()
	return nil
}

func (i *Inspector) exitIPCPortConflict(port []dockerapi.PortBinding, typ string, code int) {
	i.logger.Errorf("RunPod: port bindings comms port conflict (%s) = %#v", typ, port)

	i.xc.Out.Info("sensor.error",
		ovars{
			"message": "port binding ipc port conflict",
			"type":    typ,
		})

	i.xc.Out.State("exited",
		ovars{
			"exit.code": code,
			"component": "pod.inspector",
			"version":   v.Current(),
		})

	i.xc.Exit(code)
}

func (i *Inspector) injectSensor(localSensorPath string) error {
	// Trying to inject the sensor binary atomically using a "cp then mv" trick.

	i.logger.Debugf("RunPod: sending sensor to pod")
	out, err := i.kubectl.CpTo(
		i.ctx,
		i.pod.Namespace,
		i.pod.Name,
		sensorLoaderContainer,
		localSensorPath,
		sensorBinFileAbs+".uploading")
	if err != nil {
		i.logger.Debugf("RunPod: kubectl cp sensor.uploading -> pod failed with %q: %s", err, string(out))
		return err
	}

	out, err = i.kubectl.Exec(
		i.ctx,
		i.pod.Namespace,
		i.pod.Name,
		sensorLoaderContainer,
		"mv", sensorBinFileAbs+".uploading", sensorBinFileAbs)
	if err != nil {
		i.logger.Debugf("RunPod: kubectl exec sensor.uploading -> sensor failed with %q: %s", err, string(out))
		return err
	}

	return nil
}

func (i *Inspector) sensorConnect() error {
	sensorListenIP := "127.0.0.1"
	for _, p := range []int32{channel.CmdPort, channel.EvtPort} {
		go func(port int32) {
			cmd, _, err := i.kubectl.PortForward(
				i.ctx,
				i.pod.Namespace,
				i.pod.Name,
				sensorListenIP,
				toStringPort(port),
				toStringPort(port),
			)
			if err == nil {
				err = cmd.Wait()
			}
			if err != nil {
				// TODO: Make it fatal!
				i.logger.Debugf("RunPod: kubectl port-forward sensor port %d failed. err=%q", port, err)
			}
		}(p)
	}

	ipcClient, err := ipc.NewClient(
		sensorListenIP,
		strconv.Itoa(channel.CmdPort),
		strconv.Itoa(channel.EvtPort),
		sensor.DefaultConnectWait)
	if err != nil {
		return err
	}

	i.sensorIPCClient = ipcClient
	return nil
}

func (i *Inspector) sensorCommandStart() error {
	cmd := &command.StartMonitor{
		RTASourcePT: i.rtaSourcePT,
		AppName:     i.fatContainerCmd[0],
		KeepPerms:   i.keepPerms,
	}
	if len(i.fatContainerCmd) > 1 {
		cmd.AppArgs = i.fatContainerCmd[1:]
	}

	// if len(i.ExcludePatterns) > 0 {
	// 	cmd.Excludes = pathMapKeys(i.ExcludePatterns)
	// }

	// if len(i.PreservePaths) > 0 {
	// 	cmd.Preserves = i.PreservePaths
	// }

	// if len(i.IncludePaths) > 0 {
	// 	cmd.Includes = i.IncludePaths
	// }

	if len(i.pathPerms) > 0 {
		cmd.Perms = i.pathPerms
	}

	// if len(i.IncludeBins) > 0 {
	// 	cmd.IncludeBins = pathMapKeys(i.IncludeBins)
	// }

	// if len(i.IncludeExes) > 0 {
	// 	cmd.IncludeExes = pathMapKeys(i.IncludeExes)
	// }

	// cmd.IncludeShell = i.DoIncludeShell
	// cmd.IncludeCertAll = i.DoIncludeCertAll
	// cmd.IncludeCertBundles = i.DoIncludeCertBundles
	// cmd.IncludeCertDirs = i.DoIncludeCertDirs
	// cmd.IncludeCertPKAll = i.DoIncludeCertPKAll
	// cmd.IncludeCertPKDirs = i.DoIncludeCertPKDirs
	// cmd.IncludeNew = i.DoIncludeNew

	// if runAsUser != "" {
	// 	cmd.AppUser = runAsUser

	// 	if strings.ToLower(runAsUser) != "root" {
	// 		cmd.RunTargetAsUser = i.RunTargetAsUser
	// 	}
	// }

	// cmd.IncludeAppNextDir = i.appNodejsInspectOpts.NextOpts.IncludeAppDir
	// cmd.IncludeAppNextBuildDir = i.appNodejsInspectOpts.NextOpts.IncludeBuildDir
	// cmd.IncludeAppNextDistDir = i.appNodejsInspectOpts.NextOpts.IncludeDistDir
	// cmd.IncludeAppNextStaticDir = i.appNodejsInspectOpts.NextOpts.IncludeStaticDir
	// cmd.IncludeAppNextNodeModulesDir = i.appNodejsInspectOpts.NextOpts.IncludeNodeModulesDir

	// cmd.IncludeAppNuxtDir = i.appNodejsInspectOpts.NuxtOpts.IncludeAppDir
	// cmd.IncludeAppNuxtBuildDir = i.appNodejsInspectOpts.NuxtOpts.IncludeBuildDir
	// cmd.IncludeAppNuxtDistDir = i.appNodejsInspectOpts.NuxtOpts.IncludeDistDir
	// cmd.IncludeAppNuxtStaticDir = i.appNodejsInspectOpts.NuxtOpts.IncludeStaticDir
	// cmd.IncludeAppNuxtNodeModulesDir = i.appNodejsInspectOpts.NuxtOpts.IncludeNodeModulesDir

	// cmd.IncludeNodePackages = i.appNodejsInspectOpts.IncludePackages

	if _, err := i.sensorIPCClient.SendCommand(cmd); err != nil {
		return err
	}

	i.xc.Out.Info("cmd.startmonitor", ovars{"status": "sent"})

	for idx := 0; idx < 3; idx++ {
		evt, err := i.sensorIPCClient.GetEvent()
		if err != nil {
			if os.IsTimeout(err) || err == channel.ErrWaitTimeout {
				i.xc.Out.Info("event.startmonitor.done",
					ovars{
						"status": "receive.timeout",
					})

				i.logger.Debug("timeout waiting for the slim container to start...")
				continue
			}

			return err
		}

		if evt == nil || evt.Name == "" {
			i.logger.Debug("empty event waiting for the slim container to start (trying again)...")
			continue
		}

		if evt.Name == event.StartMonitorDone {
			i.xc.Out.Info("event.startmonitor.done",
				ovars{
					"status": "received",
				})
			return nil
		}

		if evt.Name == event.Error {
			return fmt.Errorf("start monitor error: %v", evt.Data)
		}

		if evt.Name != event.StartMonitorDone {
			i.xc.Out.Info("event.startmonitor.done",
				ovars{
					"status": "received.unexpected",
					"data":   fmt.Sprintf("%+v", evt),
				})

			//TODO: dump temp container logs
			return event.ErrUnexpectedEvent
		}
	}

	return errors.New("start monitor timeout")
}

func (i *Inspector) sensorCommandStop() error {
	resp, err := i.sensorIPCClient.SendCommand(&command.StopMonitor{})
	if err != nil {
		return err
	}
	i.logger.Debugf("'stop' monitor response => '%v'", resp)

	i.logger.Info("waiting for the pod to finish its work...")

	evt, err := i.sensorIPCClient.GetEvent()
	if err != nil {
		return err
	}
	i.logger.Debugf("sensor event => '%v'", evt)
	return nil
}

func (i *Inspector) sensorDisconnect() {
	const op = "container.Inspector.shutdownContainerChannels"
	if i.sensorIPCClient != nil {
		if err := i.sensorIPCClient.Stop(); err != nil {
			i.logger.WithFields(log.Fields{
				"op":    op,
				"error": err,
			}).Debug("shutting down channels")
		}
		i.sensorIPCClient = nil
	}
}

func findPod(
	ctx context.Context,
	client *kubernetes.Client,
	namespace string,
	targetPodLabelValue string,
) (corev1.Pod, error) {
	var pod corev1.Pod
	err := wait.PollImmediateWithContext(ctx, 1*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		pods, err := client.
			Static().
			CoreV1().
			Pods(namespace).
			List(context.TODO(), metav1.ListOptions{
				LabelSelector: targetPodLabelName + "=" + targetPodLabelValue,
			})
		if err != nil {
			return false, err
		}

		if len(pods.Items) == 0 {
			return false, nil // Keep waiting
		}
		if len(pods.Items) == 1 {
			pod = pods.Items[0]
			return true, nil // Done
		}

		return false, errors.New("unexpected - more than one target pod found")
	})

	return pod, err
}

func waitForContainer(
	ctx context.Context,
	client *kubernetes.Client,
	namespace string,
	podName string,
	contName string,
	isInit bool,
) error {
	return wait.PollImmediateWithContext(ctx, 1*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		pod, err := client.Static().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		statuses := pod.Status.ContainerStatuses
		if isInit {
			statuses = pod.Status.InitContainerStatuses
		}
		for _, contStatus := range statuses {
			if contStatus.Name == contName && contStatus.State.Running != nil {
				return true, nil
			}
		}

		return false, nil
	})
}

func fatContainerCmd(
	workload *kubernetes.Workload,
	imageInspector *image.Inspector,
	overrides *config.ContainerOverrides,
) []string {
	entrypoint := workload.TargetContainer().Command
	if len(entrypoint) == 0 {
		entrypoint = imageInspector.ImageInfo.Config.Entrypoint
	}
	cmd := workload.TargetContainer().Args
	if len(cmd) == 0 {
		cmd = imageInspector.ImageInfo.Config.Cmd
	}

	fullCmd := append(entrypoint, cmd...)
	if overrides != nil {
		if len(overrides.Entrypoint) > 0 || overrides.ClearEntrypoint {
			fullCmd = overrides.Entrypoint
			if len(overrides.Cmd) > 0 || overrides.ClearCmd {
				fullCmd = append(fullCmd, overrides.Cmd...)
			}
			//note: not using Args from PodTemplateSpec if there's an override for ENTRYPOINT
		} else {
			fullCmd = entrypoint
			if len(overrides.Cmd) > 0 || overrides.ClearCmd {
				fullCmd = append(fullCmd, overrides.Cmd...)
			} else {
				fullCmd = append(fullCmd, cmd...)
			}
		}
	}
	return fullCmd
}

func boolPtr(v bool) *bool {
	return &v
}

func toDockerPort(p int32) dockerapi.Port {
	return dockerapi.Port(fmt.Sprintf("%d/tcp", p))
}

func toStringPort(p int32) string {
	return fmt.Sprintf("%d", p)
}
