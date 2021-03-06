/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package declarative

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/loaders"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

type ManifestLoaderFunc func() ManifestController

// DefaultManifestLoader is the manifest loader we use when a manifest loader is not otherwise configured
// We curently default to loading from the filesystem, but may change this default in future
var DefaultManifestLoader ManifestLoaderFunc = func() ManifestController {
	return loaders.NewManifestLoader()
}

type reconcilerParams struct {
	rawManifestOperations []ManifestOperation
	groupVersionKind      *schema.GroupVersionKind
	objectTransformations []ObjectTransform
	manifestController    ManifestController

	prune             bool
	preserveNamespace bool

	sink       Sink
	ownerFn    OwnerSelector
	labelMaker LabelMaker
	status     Status
}

type ManifestController interface {
	// ResolveManifest returns a raw manifest as a string for a given CR object
	ResolveManifest(ctx context.Context, object runtime.Object) (string, error)
}

type Sink interface {
	// Notify tells the Sink that all objs have been created
	Notify(ctx context.Context, dest DeclarativeObject, objs *manifest.Objects) error
}

// ManifestOperation is an operation that transforms raw string manifests before applying it
type ManifestOperation = func(context.Context, DeclarativeObject, string) (string, error)

// ObjectTransform is an operation that transforms the manifest objects before applying it
type ObjectTransform = func(context.Context, DeclarativeObject, *manifest.Objects) error

// OwnerSelector selects a runtime.Object to be the owner of a given manifest.Object
type OwnerSelector = func(context.Context, DeclarativeObject, manifest.Object, manifest.Objects) (DeclarativeObject, error)

// LabelMaker returns a fixed set of labels for a given DeclarativeObject
type LabelMaker = func(context.Context, DeclarativeObject) map[string]string

// WithRawManifestOperation adds the specific ManifestOperations to the chain of manifest changes
func WithRawManifestOperation(operations ...ManifestOperation) reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.rawManifestOperations = append(p.rawManifestOperations, operations...)
		return p
	}
}

// WithObjectTransform adds the specified ObjectTransforms to the chain of manifest changes
func WithObjectTransform(operations ...ObjectTransform) reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.objectTransformations = append(p.objectTransformations, operations...)
		return p
	}
}

// WithGroupVersionKind specifies the GroupVersionKind of the managed custom resource
// This option is required.
func WithGroupVersionKind(gvk schema.GroupVersionKind) reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.groupVersionKind = &gvk
		return p
	}
}

// WithManifestController overrides the default source for loading manifests
func WithManifestController(mc ManifestController) reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.manifestController = mc
		return p
	}
}

func withImageRegistryTransform(privateRegistry, imagePullSecret string) func(context.Context, DeclarativeObject, *manifest.Objects) error {
	return func(c context.Context, o DeclarativeObject, m *manifest.Objects) error {
		return applyImageRegistry(c, o, m, privateRegistry, imagePullSecret)
	}
}

func applyImageRegistry(ctx context.Context, operatorObject DeclarativeObject, manifest *manifest.Objects, registry, secret string) error {
	if registry == "" && secret == "" {
		return nil
	}
	for _, manifestItem := range manifest.Items {
		if manifestItem.Kind == "Deployment" || manifestItem.Kind == "DaemonSet" ||
			manifestItem.Kind == "StatefulSet" || manifestItem.Kind == "Job" ||
			manifestItem.Kind == "CronJob" {
			if registry != "" {
				if err := manifestItem.MutateContainers(applyPrivateRegistryToContainer(registry)); err != nil {
					return fmt.Errorf("error applying private registry: %v", err)
				}
			}
			if secret != "" {
				if err := manifestItem.MutatePodSpec(applyImagePullSecret(secret)); err != nil {
					return fmt.Errorf("error applying image pull secret: %v", err)
				}
			}
		}
	}
	return nil
}

func applyImagePullSecret(secret string) func(map[string]interface{}) error {
	return func(podSpec map[string]interface{}) error {
		imagePullSecret := map[string]interface{}{"name": secret}
		if err := unstructured.SetNestedSlice(podSpec, []interface{}{imagePullSecret}, "imagePullSecrets"); err != nil {
			return fmt.Errorf("error applying pull image secret: %v", err)
		}
		return nil
	}
}

func applyPrivateRegistryToContainer(registry string) func(map[string]interface{}) error {
	return func(container map[string]interface{}) error {
		image, _, err := unstructured.NestedString(container, "image")
		if err != nil {
			return fmt.Errorf("error reading container image: %v", err)
		}
		parts := strings.Split(image, "/")
		imageName := parts[len(parts)-1]
		container["image"] = registry + "/" + imageName
		return nil
	}
}

// WithApplyPrune turns on the --prune behavior of kubectl apply. This behavior deletes any
// objects that exist in the API server that are not deployed by the current version of the manifest
// which match a label specific to the addon instance.
//
// This option requires WithLabels to be used
func WithApplyPrune() reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.prune = true
		return p
	}
}

// WithOwner sets an owner ref on each deployed object by the OwnerSelector
func WithOwner(ownerFn OwnerSelector) reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.ownerFn = ownerFn
		return p
	}
}

// WithLabels sets a fixed set of labels configured provided by a LabelMaker
// to all deployment objecs for a given DeclarativeObject
func WithLabels(labelMaker LabelMaker) reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.labelMaker = labelMaker
		return p
	}
}

// WithStatus provides a Status interface that will be used during Reconcile
func WithStatus(status Status) reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.status = status
		return p
	}
}

// WithPreserveNamespace preserves the namespaces defined in the deployment manifest
// instead of matching the namespace of the DeclarativeObject
func WithPreserveNamespace() reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.preserveNamespace = true
		return p
	}
}

// WithManagedApplication is a transform that will modify the Application object
// in the deployment to match the configuration of the rest of the deployment.
func WithManagedApplication(labelMaker LabelMaker) reconcilerOption {
	return func(p reconcilerParams) reconcilerParams {
		p.objectTransformations = append(p.objectTransformations, func(ctx context.Context, instance DeclarativeObject, objects *manifest.Objects) error {
			return transformApplication(ctx, instance, objects, labelMaker)
		})
		return p
	}
}

type reconcilerOption func(params reconcilerParams) reconcilerParams
