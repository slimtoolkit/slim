package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
)

// HandleKubernetesRuntime implements support for the k8s runtime
func HandleKubernetesRuntime(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *commands.GenericParams,
	commandParams *CommandParams) {
	logger = logger.WithField("op", "debug.HandleKubernetesRuntime")

	cpJson, _ := json.Marshal(commandParams)
	logger.WithField("cparams", string(cpJson)).Trace("call")
	defer logger.Trace("exit")

	ecName := generateContainerName()
	ctx := context.Background()

	api, restConfig, err := apiClientFromConfig(commandParams.Kubeconfig)
	if err != nil {
		logger.WithError(err).Error("apiClientFromConfig")
		xc.FailOn(err)
	}

	nsName, err := ensureNamespace(ctx, api, commandParams.TargetNamespace)
	if err != nil {
		logger.WithError(err).Error("ensureNamespace")
		xc.FailOn(err)
	}

	pod, podName, err := ensurePod(ctx, api, nsName, commandParams.TargetPod)
	if apierrors.IsNotFound(err) {
		logger.WithError(err).
			WithFields(log.Fields{
				"ns":  nsName,
				"pod": podName,
			}).Error("ensurePod - not found")
		xc.FailOn(err)
	} else if statusError, isStatus := err.(*apierrors.StatusError); isStatus {
		logger.WithError(err).
			WithFields(log.Fields{
				"ns":     nsName,
				"pod":    podName,
				"status": statusError.ErrStatus.Message,
			}).Error("ensurePod - status error")
		xc.FailOn(err)
	} else if err != nil {
		logger.WithError(err).
			WithFields(log.Fields{
				"ns":     nsName,
				"pod":    podName,
				"status": statusError.ErrStatus.Message,
			}).Error("ensurePod - other error")
		xc.FailOn(err)
	}

	logger.WithField("phase", pod.Status.Phase).Debug("target pod status")

	if pod.Status.Phase != corev1.PodRunning {
		logger.Error("target pod is not running")
		xc.FailOn(fmt.Errorf("target pod is not running"))
	}

	logger.WithFields(
		log.Fields{
			"ns":       nsName,
			"pod":      podName,
			"ec.count": len(pod.Spec.EphemeralContainers),
		}).Debug("target pod info")

	if commandParams.TargetRef == "" {
		logger.Debug("no explicit target container... pick one")
		//TODO: improve this logic (to also check for the default container)
		if len(pod.Spec.Containers) > 0 {
			commandParams.TargetRef = pod.Spec.Containers[0].Name
		} else {
			xc.FailOn(fmt.Errorf("no containers"))
		}
	}

	logger.WithField("target", commandParams.TargetRef).Debug("locating container")

	targetContainerIndex := -1
	targetContainerIsRunning := false
	var targetContainer *corev1.Container
	for i, c := range pod.Spec.Containers {
		if c.Name == commandParams.TargetRef {
			targetContainerIndex = i
			targetContainer = &c

			logger.WithFields(
				log.Fields{
					"index":  targetContainerIndex,
					"ns":     nsName,
					"pod":    podName,
					"target": commandParams.TargetRef,
				}).Trace("found container")
			break
		}
	}

	if targetContainer != nil {
		//targetContainer.TTY
		//might be good to configure the ephemeral container TTY to match the target container TTY
		logger.WithField("data", fmt.Sprintf("%#v", targetContainer)).Trace("target container info")
	}

	containerFound := false
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == commandParams.TargetRef {
			containerFound = true
			if containerStatus.State.Running != nil {
				targetContainerIsRunning = true
				logger.Trace("target container is running")
			}
			break
		}
	}

	if !containerFound {
		logger.Errorf("Container %s not found in pod %s", commandParams.TargetRef, podName)
		xc.FailOn(fmt.Errorf("target container not found"))
	}

	if !targetContainerIsRunning {
		logger.Debugf("Container %s is not running in pod %s (wait)", commandParams.TargetRef, podName)
		err = waitForContainer(ctx, api, nsName, podName, commandParams.TargetRef, ctStandard)
		if err != nil {
			logger.WithError(err).Error("waitForContainer")
			xc.FailOn(err)
		}
	}

	var workDir string
	isEcPrivileged := true
	//TODO: pass commandParams.DoTerminal
	ecInfo := newEphemeralContainerInfo(
		commandParams.TargetRef,
		ecName,
		commandParams.DebugContainerImage,
		commandParams.Entrypoint,
		commandParams.Cmd,
		workDir,
		isEcPrivileged)

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ecInfo)

	_, err = api.CoreV1().
		Pods(pod.Namespace).
		UpdateEphemeralContainers(ctx, pod.Name, pod, metav1.UpdateOptions{})

	if err != nil {
		logger.WithError(err).Error("error adding the ephemeral container to target pod")
		xc.FailOn(err)
	}

	updatedPod, err := api.CoreV1().
		Pods(nsName).
		Get(ctx, commandParams.TargetPod, metav1.GetOptions{})
	if err != nil {
		logger.WithError(err).Error("error getting the ephemeral container from target pod")
		xc.FailOn(err)
	}

	doTTY := ecInfo.TTY

	logger.WithFields(
		log.Fields{
			"ns":        nsName,
			"pod":       podName,
			"target":    commandParams.TargetRef,
			"ephemeral": ecName,
			"image":     commandParams.DebugContainerImage,
		}).Debug("attached ephemeral container")

	ec := ephemeralContainerFromPod(updatedPod, commandParams.TargetRef, ecName)
	if ec == nil {
		logger.Errorf("ephemeral container not found in pod - ", ecName)
		xc.FailOn(fmt.Errorf("ephemeral container not found"))
	}

	ecData, _ := json.Marshal(ec)
	logger.WithField("data", string(ecData)).Trace("ephemeral container")

	var ecContainerIsRunning bool
	for _, ecStatus := range updatedPod.Status.EphemeralContainerStatuses {
		if ecStatus.Name == ecName {
			if ecStatus.State.Running != nil {
				ecContainerIsRunning = true
			}
			break
		}
	}

	if !ecContainerIsRunning {
		logger.Debugf("EC container %s is not running in pod %s", ecName, podName)
		err = waitForContainer(ctx, api, nsName, podName, ecName, ctEphemeral)
		if err != nil {
			logger.WithError(err).Error("waitForContainer")
			xc.FailOn(err)
		}
	}

	req := api.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(nsName).
		SubResource("attach").
		VersionedParams(&corev1.PodAttachOptions{
			Container: ecName,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       doTTY,
		}, scheme.ParameterCodec)

	logger.Tracef("pod attach request URL:", req.URL())

	attach, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, req.URL())
	if err != nil {
		logger.WithError(err).Error("remotecommand.NewSPDYExecutor")
		xc.FailOn(err)
	}

	xc.Out.Info("terminal.start",
		ovars{
			"note": "press enter if you dont see any output",
		})

	logger.Trace("starting stream...")
	//TODO:
	//use commandParams.DoTerminal to conditionally enable the interactive terminal
	//if false configure stream to do a one off command execution
	//and dump the container logs
	//similar to how it's done with the docker runtime

	fmt.Printf("\n")
	//note: blocks until done streaming or failure...
	err = attach.StreamWithContext(
		ctx,
		remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Tty:    doTTY,
		})

	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.WithError(err).
				WithField("ec", ecName).
				Error("attach.StreamWithContext - not found")
		} else if statusError, isStatus := err.(*apierrors.StatusError); isStatus {
			logger.WithError(err).
				WithFields(log.Fields{
					"ec":     ecName,
					"status": statusError.ErrStatus.Message,
				}).Error("attach.StreamWithContext - status error")
		} else {
			logger.WithError(err).
				WithField("ec", ecName).
				Error("attach.StreamWithContext - other error")
		}

		xc.FailOn(err)
	}

	//TODO: extra feature - connect to existing/previous ephemeral container instances
	//need an ability to list all ephemeral containers in a pod, all ECs for a specific target container
	//for the listed ephemeral containers need to show time attached and custom params (like entrypoint, cmd)
}

func ensureNamespace(ctx context.Context, api *kubernetes.Clientset, name string) (string, error) {
	if name == "" {
		namespaces, err := api.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", err
		}

		if len(namespaces.Items) == 0 {
			return "", fmt.Errorf("no namespaces")
		}

		return namespaces.Items[0].Name, nil
	}

	_, err := api.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debugf("ensureNamespace: %s namespace is not found", name)
		}

		return "", err
	}

	return name, nil
}

func ensurePod(ctx context.Context, api *kubernetes.Clientset, nsName string, podName string) (*corev1.Pod, string, error) {
	if podName == "" {
		pods, err := api.CoreV1().Pods(nsName).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, "", err
		}

		if len(pods.Items) == 0 {
			return nil, "", fmt.Errorf("no pods")
		}

		podName = pods.Items[0].Name
	}

	var outputPod *corev1.Pod
	isPodRunning := func() (bool, error) {
		pod, err := api.CoreV1().Pods(nsName).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		switch pod.Status.Phase {
		case corev1.PodRunning:
			outputPod = pod
			return true, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, fmt.Errorf("pod is done")
		}
		return false, nil
	}

	err := wait.PollImmediate(2*time.Second, 2*time.Minute, isPodRunning)
	if err != nil {
		return nil, "", err
	}

	return outputPod, podName, nil
}

const (
	ctInit      = "init"
	ctStandard  = "standard"
	ctEphemeral = "ephemeral"
)

func waitForContainer(ctx context.Context, api *kubernetes.Clientset, nsName string, podName string, containerName string, containerType string) error {
	log.Debugf("waitForContainer(%s,%s,%s,%s)", nsName, podName, containerName, containerType)

	isContainerRunning := func() (bool, error) {
		pod, err := api.CoreV1().Pods(nsName).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		switch pod.Status.Phase {
		case corev1.PodRunning:
			var statuses []corev1.ContainerStatus
			switch containerType {
			case ctInit:
				statuses = pod.Status.InitContainerStatuses
			case ctStandard:
				statuses = pod.Status.ContainerStatuses
			case ctEphemeral:
				statuses = pod.Status.EphemeralContainerStatuses
			default:
				return false, fmt.Errorf("unknown container type")
			}

			log.Tracef("waitForContainer: statuses - \n", len(statuses))
			for _, status := range statuses {
				if status.Name == containerName {
					if status.State.Running != nil {
						log.Tracef("waitForContainer: RUNNING - %s/%s/%s[%s]", nsName, podName, containerName, containerType)
						return true, nil
					} else {
						log.Trace("waitForContainer: target is not running yet...")
					}
				}
			}

			//don't fail right away, let it time out...
			return false, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, fmt.Errorf("pod is done")
		}

		return false, nil
	}

	return wait.PollImmediate(2*time.Second, 4*time.Minute, isContainerRunning)
}

const (
	containerNamePat = "ds-debugger-%v-%v"
)

func generateContainerName() string {
	return fmt.Sprintf(containerNamePat, os.Getpid(), time.Now().UTC().Format("20060102150405"))
}

func ephemeralContainerFromPod(
	pod *corev1.Pod,
	target string,
	name string) *corev1.EphemeralContainer {
	for _, ec := range pod.Spec.EphemeralContainers {
		if ec.TargetContainerName == target &&
			ec.Name == name {
			return &ec
		}
	}

	return nil
}

func newEphemeralContainerInfo(
	target string, // target container in the pod
	name string, // name to use for the ephemeral container (must be unique)
	image string, // image to use for the ephemeral container
	command []string, // custom ENTRYPOINT to use for the ephemeral container (yes, it's not CMD :-))
	args []string, // custom CMD to use
	workingDir string,
	isPrivileged bool, // true if it should be a privileged container
) corev1.EphemeralContainer {
	isTrue := true
	out := corev1.EphemeralContainer{
		TargetContainerName: target,
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			TTY:        true,
			Stdin:      true,
			Name:       name,
			Image:      image,
			Command:    command,
			Args:       args,
			WorkingDir: workingDir,
			//TODO: add support for more params
		},
	}

	if isPrivileged {
		out.EphemeralContainerCommon.SecurityContext = &corev1.SecurityContext{
			Privileged: &isTrue,
		}
	}

	return out
}

func apiClientFromConfig(kubeconfig string) (*kubernetes.Clientset, *restclient.Config, error) {
	kubeconfig = os.ExpandEnv(kubeconfig)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return clientset, config, nil
}
