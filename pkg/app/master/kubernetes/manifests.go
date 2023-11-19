package kubernetes

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/slimtoolkit/slim/pkg/app/master/config"
)

// TODO: Start supporting CRDs - currently the hardcoded client-go scheme usage
//       will likely make the manifests containing CRDs fail.

type Manifests struct {
	infos []*resource.Info

	client            *Client
	resourceBuilderFn ResourceBuilderFunc
}

func ManifestsFromFiles(
	opts config.KubernetesOptions,
	client *Client,
	resourceBuilderFn ResourceBuilderFunc,
) (*Manifests, error) {
	namespace := opts.Target.Namespace
	if namespace == "" {
		namespace = namespaceDefault
	}

	infos, err := resourceBuilderFn().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(namespace).
		DefaultNamespace().
		FilenameParam(
			false,
			&resource.FilenameOptions{
				Filenames: opts.Manifests,
				Recursive: true,
			}).
		Do().
		Infos()
	if err != nil {
		return nil, err
	}

	return &Manifests{
		infos:             infos,
		client:            client,
		resourceBuilderFn: resourceBuilderFn,
	}, nil
}

func (ms *Manifests) Find(target config.KubernetesTarget) (*resource.Info, error) {
	mapping, err := ms.resourceBuilderFn().
		Local().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam("not-really-used").
		ResourceTypeOrNameArgs(true, target.Workload).
		SingleResourceType().
		Do().
		ResourceMapping()
	if err != nil {
		return nil, err
	}

	name, err := target.WorkloadName()
	if err != nil {
		return nil, err
	}

	matches := []*resource.Info{}
	for _, info := range ms.infos {
		if info.Mapping.GroupVersionKind.GroupKind() != mapping.GroupVersionKind.GroupKind() {
			continue
		}
		if info.Name != name {
			continue
		}
		if target.Namespace != "" && target.Namespace != info.Namespace {
			continue
		}

		matches = append(matches, info)
	}

	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("found more than 1 workload '%s/%s' match in the supplied manifests", target.Namespace, name)
	}

	return matches[0], nil
}

func (ms *Manifests) Apply(ctx context.Context, predicate func(info *resource.Info) bool) error {
	for _, info := range ms.infos {
		if !predicate(info) {
			continue
		}

		if err := ms.client.CreateOrUpdate(ctx, info); err != nil {
			// TODO: consider partial applying
			return err
		}
	}

	return nil
}

func (ms *Manifests) Delete(ctx context.Context) error {
	for _, info := range ms.infos {
		if err := ms.client.Delete(ctx, info); err != nil && !apierrors.IsNotFound(err) {
			// TODO: consider partial deletion
			return err
		}
	}

	return nil
}
