/*
Copyright 2018 The Kubernetes Authors.

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

package dashboard

import (
	//applicationv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	api "sigs.k8s.io/kubebuilder-declarative-pattern/examples/dashboard-operator/pkg/apis/addons/v1alpha1"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/status"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
)

var _ reconcile.Reconciler = &ReconcileDashboard{}

// ReconcileDashboard reconciles a Dashboard object
type ReconcileDashboard struct {
	declarative.Reconciler
}

// newReconciler returns a initialized ReconcileDashboard
func newReconciler(mgr manager.Manager) (*ReconcileDashboard, declarative.LabelMaker) {
	labels := map[string]string{
		"k8s-app": "kubernetes-dashboard",
	}

	r := &ReconcileDashboard{}

	srcLabels := declarative.SourceLabel(mgr.GetScheme())

	err := r.Reconciler.Init(mgr, &api.Dashboard{}, "dashboard",
		declarative.WithObjectTransform(declarative.AddLabels(labels)),
		declarative.WithOwner(declarative.SourceAsOwner),
		declarative.WithLabels(srcLabels),
		declarative.WithStatus(status.NewBasic(mgr.GetClient())),
		declarative.WithPreserveNamespace(),
		declarative.WithManagedApplication(srcLabels),
		declarative.WithObjectTransform(addon.TransformApplicationFromStatus),
		declarative.WithApplyPrune(),
	)

	if err != nil {
		panic(err)
	}

	return r, srcLabels
}

func Add(mgr manager.Manager) error {
	r, srcLabels := newReconciler(mgr)

	c, err := controller.New("dashboard-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Dashboard
	err = c.Watch(&source.Kind{Type: &api.Dashboard{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to deployed objects
	_, err = declarative.WatchAll(mgr.GetConfig(), c, r, srcLabels)
	if err != nil {
		return err
	}

	return nil
}
