package kubernetes

import (
	"errors"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/slimtoolkit/slim/pkg/app/master/config"
)

const (
	namespaceDefault           = "default"
	annotationDefaultContainer = "kubectl.kubernetes.io/default-container"
)

type Workload struct {
	info                *resource.Info
	orig                runtime.Object
	targetContainerName string
}

func newWorkload(info *resource.Info, targetContainerName string) *Workload {
	return &Workload{
		info:                info,
		orig:                info.Object.DeepCopyObject(),
		targetContainerName: targetContainerName,
	}
}

func (w *Workload) Namespace() string {
	return w.info.Namespace
}

func (w *Workload) Name() string {
	return w.info.Name
}

func (w *Workload) Info() *resource.Info {
	return w.info
}

func (w *Workload) Template() *corev1.PodTemplateSpec {
	switch obj := w.info.Object.(type) {
	case *appsv1.DaemonSet:
		return &obj.Spec.Template
	case *appsv1.Deployment:
		return &obj.Spec.Template
	case *appsv1.ReplicaSet:
		return &obj.Spec.Template
	case *appsv1.StatefulSet:
		return &obj.Spec.Template
	case *batchv1.Job:
		return &obj.Spec.Template
	case *batchv1.CronJob:
		return &obj.Spec.JobTemplate.Spec.Template
	default:
		// Shouldn't really happen...
		return nil
	}
}

func (w *Workload) Container(name string) *corev1.Container {
	for _, c := range w.Template().Spec.Containers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func (w *Workload) DefaultContainer() *corev1.Container {
	as := w.Template().Annotations
	if as != nil && as[annotationDefaultContainer] != "" {
		return w.Container(as[annotationDefaultContainer])
	}

	if len(w.Template().Spec.Containers) == 1 {
		return &w.Template().Spec.Containers[0]
	}

	// No default
	return nil
}

func (w *Workload) TargetContainer() *corev1.Container {
	if w.targetContainerName == "" {
		return w.DefaultContainer()
	}
	return w.Container(w.targetContainerName)
}

func (w *Workload) AddEmptyDirVolume(name string) {
	w.Template().Spec.Volumes = append(w.Template().Spec.Volumes,
		corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
}

func (w *Workload) AddInitContainer(cont corev1.Container) {
	w.Template().Spec.InitContainers = append(w.Template().Spec.InitContainers, cont)
}

func (w *Workload) SetReplicasIfApplicable(n int32) bool {
	switch obj := w.info.Object.(type) {
	case *appsv1.Deployment:
		obj.Spec.Replicas = &n
	case *appsv1.ReplicaSet:
		obj.Spec.Replicas = &n
	case *appsv1.StatefulSet:
		obj.Spec.Replicas = &n
	default:
		return false
	}
	return true
}

func (w *Workload) ResetChanges() {
	w.info.Object = w.orig.DeepCopyObject()
	w.info.ResourceVersion = ""
}

// WorkloadFinder searches for the workload object in:
//   - supplied manifest files (if any)
//   - default cluster
type WorkloadFinder struct {
	manifests         *Manifests
	resourceBuilderFn ResourceBuilderFunc
}

func NewWorkloadFinder(manifests *Manifests, resourceBuilderFn ResourceBuilderFunc) *WorkloadFinder {
	return &WorkloadFinder{
		manifests:         manifests,
		resourceBuilderFn: resourceBuilderFn,
	}
}

func (f *WorkloadFinder) Find(target config.KubernetesTarget) (*Workload, error) {
	info, err := func() (*resource.Info, error) {
		if f.manifests != nil {
			return f.manifests.Find(target)
		}
		return f.findInCluster(target)
	}()
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return newWorkload(info, target.Container), nil
}

func (f *WorkloadFinder) findInCluster(target config.KubernetesTarget) (*resource.Info, error) {
	namespace := target.Namespace
	if namespace == "" {
		namespace = namespaceDefault
	}

	infos, err := f.resourceBuilderFn().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(namespace).
		SingleResourceType().
		ResourceTypeOrNameArgs(true, target.Workload).
		Do().
		Infos()
	if err != nil {
		return nil, err
	}

	if len(infos) == 0 {
		return nil, errors.New("couldn't find workload in cluster")
	}
	if len(infos) > 1 {
		return nil, errors.New("couldn't unambiguously identify workload in cluster")
	}

	return infos[0], nil
}
