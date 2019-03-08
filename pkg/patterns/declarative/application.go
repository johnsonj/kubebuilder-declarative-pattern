// application.go manages an Application[1]
//
// [1] https://github.com/kubernetes-sigs/application
package declarative

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

func transformApplication(ctx context.Context, instance DeclarativeObject, objects *manifest.Objects, labelMaker LabelMaker) error {
	log := log.Log

	var app *manifest.Object
	for _, o := range objects.Items {
		if o.Group == "app.k8s.io" && o.Kind == "Application" {
			if app != nil {
				log.Info("cannot update application with multiple app.k8s.io/Application in manifest")
				return nil
			}
			app = o
		}
	}

	if app == nil {
		log.Info("cannot transformApplication without an app.k8s.io/Application in the manifest")
		return nil
	}

	app.SetNestedFieldNoCopy(metav1.LabelSelector{MatchLabels: labelMaker(ctx, instance)}, "spec", "selector")
	app.SetNestedFieldNoCopy(uniqueGroupVersionKind(objects), "spec", "componentGroupKinds")

	return nil
}

// uniqueGroupKind returns all unique GroupKind defined in objects
func uniqueGroupKind(objects *manifest.Objects) []metav1.GroupKind {
	kinds := map[metav1.GroupKind]struct{}{}
	for _, o := range objects.Items {
		gk := o.GroupKind()
		kinds[metav1.GroupKind{Group: gk.Group, Kind: gk.Kind}] = struct{}{}
	}
	var unique []metav1.GroupKind
	for gk := range kinds {
		unique = append(unique, gk)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].String() < unique[i].String()
	})
	return unique
}
