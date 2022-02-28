/*
Copyright 2022.

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

package controllers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NetworkPolicyReconciler reconciles a NetworkPolicy object
type NetworkPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *NetworkPolicyReconciler) findRelevantPodsFromSelector(ctx context.Context, ls *metav1.LabelSelector) ([]corev1.Pod, error) {
	pods := &corev1.PodList{}

	s, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil, err
	}

	err = r.List(ctx, pods, client.MatchingLabelsSelector{Selector: s})
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func (r *NetworkPolicyReconciler) findRelevantPodsFromNamespaceSelector(ctx context.Context, ls *metav1.LabelSelector) ([]corev1.Pod, error) {
	s, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil, err
	}

	nss := corev1.NamespaceList{}

	err = r.List(ctx, &nss, client.MatchingLabelsSelector{Selector: s})
	if err != nil {
		return nil, err
	}

	var pods []corev1.Pod
	for _, ns := range nss.Items {
		foundPods := &corev1.PodList{}
		err := r.List(ctx, foundPods, client.InNamespace(ns.GetName()))
		if err != nil {
			// TODO need error handling?
			continue
		}
		pods = append(pods, foundPods.Items...)
	}

	return pods, nil
}

//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=k8s.io,resources=pods/finalizers,verbs=get;list;watch
//+kubebuilder:rbac:groups=k8s.io,resources=namespaces/finalizers,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NetworkPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *NetworkPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxLog := log.FromContext(ctx)

	netpol := &netv1.NetworkPolicy{}

	err := r.Get(ctx, req.NamespacedName, netpol)
	if err != nil {
		ctxLog.Info("netpol not found")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ctxLog.Info("netpol found")

	// process spec.podSelector
	var targetPods []corev1.Pod
	if len(netpol.Spec.PodSelector.MatchExpressions) != 0 {
		foundPods, err := r.findRelevantPodsFromSelector(ctx, &netpol.Spec.PodSelector)
		if err != nil {
			ctxLog.Info("target pods not found")
		}
		targetPods = append(targetPods, foundPods...)
	} else {
		// TODO support empty selector
		panic("empty podSelector is not supported yet")
	}

	ctxLog.Info("target pods found")
	for _, pod := range targetPods {
		ctxLog.Info("target pod", "name", pod.GetName(), "ip", pod.Status.PodIPs)
	}

	// process spec.ingress
	var ingressPods []corev1.Pod
	for _, ingress := range netpol.Spec.Ingress {
		for _, from := range ingress.From {
			// TODO support ipBlock

			// TODO support NamespaceSelector

			if from.PodSelector != nil {
				psPods, err := r.findRelevantPodsFromSelector(ctx, from.PodSelector)
				if err != nil {
					return ctrl.Result{}, nil
				}
				ingressPods = append(ingressPods, psPods...)
			}
		}

		// TODO support ports
	}

	// TODO support egress

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1.NetworkPolicy{}).
		Complete(r)
}
