package kubernetes

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/slimtoolkit/slim/pkg/app/master/config"
)

type Client struct {
	dynamic dynamic.Interface
	static  kubernetes.Interface
}

func NewClient(kubeOpts config.KubernetesOptions) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeOpts.Kubeconfig)
	if err != nil {
		return nil, err
	}

	client := Client{}

	client.dynamic, err = dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	client.static, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func (c *Client) Dynamic() dynamic.Interface {
	return c.dynamic
}

func (c *Client) Static() kubernetes.Interface {
	return c.static
}

func (c *Client) CreateOrUpdate(ctx context.Context, info *resource.Info) (err error) {
	dto := unstructured.Unstructured{}
	dto.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(info.Object)
	if err != nil {
		return
	}

	// Disable the version check since we don't expect concurrent changes.
	dto.SetResourceVersion("")

	var obj *unstructured.Unstructured
	obj, err = c.dynamic.
		Resource(info.Mapping.Resource).
		Namespace(info.Namespace).
		Create(ctx, &dto, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		obj, err = c.dynamic.
			Resource(info.Mapping.Resource).
			Namespace(info.Namespace).
			Update(ctx, &dto, metav1.UpdateOptions{})
	}

	if err != nil {
		return
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, info.Object)
	if err == nil {
		info.ResourceVersion = obj.GetResourceVersion()
	}

	return
}

func (c *Client) Delete(ctx context.Context, info *resource.Info) error {
	return c.dynamic.
		Resource(info.Mapping.Resource).
		Namespace(info.Namespace).
		Delete(
			ctx,
			info.Name,
			metav1.DeleteOptions{})
}

// ResourceBuilderFunc is a helper function to avoid
// passing KuberneteOptions through unrelated code.
type ResourceBuilderFunc func() *resource.Builder

func NewResourceBuilder(kubeOpts config.KubernetesOptions) *resource.Builder {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.KubeConfig = &kubeOpts.Kubeconfig
	return resource.NewBuilder(configFlags)
}

func NewResourceBuilderFunc(kubeOpts config.KubernetesOptions) ResourceBuilderFunc {
	return func() *resource.Builder {
		return NewResourceBuilder(kubeOpts)
	}
}
