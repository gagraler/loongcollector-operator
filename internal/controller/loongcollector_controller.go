/*
Copyright 2025 LoongCollector Sigs.

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

package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	loongcollectorv1alpha1 "github.com/infraflows/loongcollector-operator/api/v1alpha1"
	"github.com/infraflows/loongcollector-operator/internal/pkg/kube"
)

const (
	// loongcollectorFinalizer 是 LoongCollector 资源的 finalizer 名称
	loongcollectorFinalizer = "loongcollector.finalizers.infraflow.co"
	// SecretName 是 LoongCollector 配置的 secret 名称
	SecretName = "loongcollector-config"
)

// LoongCollectorReconciler reconciles a LoongCollector object
type LoongCollectorReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Event  record.EventRecorder
}

//+kubebuilder:rbac:groups=loongcollector.loong.io,resources=loongcollectors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=loongcollector.loong.io,resources=loongcollectors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=loongcollector.loong.io,resources=loongcollectors/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
func (r *LoongCollectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.WithValues("loongcollector", req.NamespacedName)

	loongCollector := &loongcollectorv1alpha1.LoongCollector{}
	err := r.Get(ctx, req.NamespacedName, loongCollector)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("LoongCollector resource not found, it may have been deleted")
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "failed to get LoongCollector")
		return ctrl.Result{}, err
	}

	if loongCollector.DeletionTimestamp != nil {

		err := kube.HandleFinalizerWithCleanup(ctx, r.Client, loongCollector, loongcollectorFinalizer, r.Log, r.cleanupResource)
		return ctrl.Result{}, err
	}

	if err := r.reconcileSecret(ctx, loongCollector); err != nil {
		r.Log.Error(err, "failed to reconcile secret")
		return ctrl.Result{}, err
	} else {
		r.Log.Info("secret reconciled")
		r.Event.Event(loongCollector, corev1.EventTypeNormal, "Reconciled", "Reconciled Secret")
	}

	daemonSet := &appsv1.DaemonSet{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      loongCollector.Name,
		Namespace: loongCollector.Namespace,
	}, daemonSet)

	if err != nil && errors.IsNotFound(err) {
		daemonSet = r.reconcileDaemonSet(loongCollector)
		if err := r.Create(ctx, daemonSet); err != nil {
			r.Log.Error(err, "failed to create DaemonSet")
			return ctrl.Result{}, err
		}
		r.Log.Info("successfully created DaemonSet")
		r.Event.Event(loongCollector, corev1.EventTypeNormal, "Created", "Created DaemonSet")
		return ctrl.Result{}, nil
	}

	updatedDaemonSet := r.reconcileDaemonSet(loongCollector)
	if err := r.Update(ctx, updatedDaemonSet); err != nil {
		r.Log.Error(err, "failed to update DaemonSet")
		return ctrl.Result{}, err
	}

	loongCollector.Status.Replicas = daemonSet.Status.DesiredNumberScheduled
	loongCollector.Status.AvailableReplicas = daemonSet.Status.NumberAvailable
	if err := r.Status().Update(ctx, loongCollector); err != nil {
		r.Log.Error(err, "failed to update LoongCollector status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// cleanupResource 处理资源删除
func (r *LoongCollectorReconciler) cleanupResource(ctx context.Context, cr *loongcollectorv1alpha1.LoongCollector) error {

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      SecretName,
		Namespace: cr.Namespace,
	}, secret)
	if err == nil {
		if err := r.Delete(ctx, secret); err != nil {
			r.Log.Error(err, "failed to delete secret")
			return err
		}
		r.Log.Info("successfully deleted secret")
	}

	daemonSet := &appsv1.DaemonSet{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      cr.Name,
		Namespace: cr.Namespace,
	}, daemonSet)
	if err == nil {
		if err := r.Delete(ctx, daemonSet); err != nil {
			r.Log.Error(err, "failed to delete daemonset")
			return err
		}
		r.Log.Info("successfully deleted daemonset")
	}

	if controllerutil.ContainsFinalizer(cr, loongcollectorFinalizer) {
		controllerutil.RemoveFinalizer(cr, loongcollectorFinalizer)
		if err := r.Update(ctx, cr); err != nil {
			r.Log.Error(err, "failed to remove finalizer")
			return err
		}
	}

	return nil
}

// reconcileSecret 创建或更新 Secret
func (r *LoongCollectorReconciler) reconcileSecret(ctx context.Context, cr *loongcollectorv1alpha1.LoongCollector) error {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-secret",
			Namespace: cr.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{},
	}

	// 如果配置了 SLS，添加访问凭证
	if cr.Spec.Env != nil {
		for _, env := range cr.Spec.Env {
			if env.Name == "default_access_key_id" {
				secret.Data["access_key_id"] = []byte(env.Value)
			}
			if env.Name == "default_access_key" {
				secret.Data["access_key"] = []byte(env.Value)
			}
		}
	}

	if err := ctrl.SetControllerReference(cr, secret, r.Scheme); err != nil {
		r.Log.Error(err, "failed to set controller reference for secret")
		return err
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, secret, func() error {
		return nil
	})
	if err != nil {
		r.Log.Error(err, "failed to create or update secret")
		return err
	}

	r.Log.Info("secret reconciled", "operation", op)
	return nil
}

// reconcileDaemonSet reconcile DaemonSet
func (r *LoongCollectorReconciler) reconcileDaemonSet(cr *loongcollectorv1alpha1.LoongCollector) *appsv1.DaemonSet {
	labels := map[string]string{
		"app": cr.Name,
	}

	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            cr.Name,
							Image:           cr.Spec.Image,
							ImagePullPolicy: cr.Spec.ImagePullPolicy,
							Resources:       cr.Spec.Resources,
							Env:             cr.Spec.Env,
							VolumeMounts:    cr.Spec.VolumeMounts,
							Lifecycle:       cr.Spec.Lifecycle,
							LivenessProbe:   cr.Spec.LivenessProbe,
							ReadinessProbe:  cr.Spec.ReadinessProbe,
							StartupProbe:    cr.Spec.StartupProbe,
							SecurityContext: cr.Spec.SecurityContext,
						},
					},
					Volumes:         cr.Spec.Volumes,
					Tolerations:     cr.Spec.Tolerations,
					DNSPolicy:       cr.Spec.DNSPolicy,
					HostNetwork:     cr.Spec.HostNetwork,
					SecurityContext: cr.Spec.PodSecurityContext,
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(cr, daemonSet, r.Scheme); err != nil {
		r.Log.Error(err, "failed to set controller reference")
	}

	return daemonSet
}

// SetupWithManager sets up the controller with the Manager.
func (r *LoongCollectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&loongcollectorv1alpha1.LoongCollector{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
