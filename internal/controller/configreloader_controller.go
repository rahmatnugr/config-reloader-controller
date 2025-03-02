/*
Copyright 2025.

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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	// "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	configreloaderv1alpha1 "github.com/rahmatnugr/config-reloader-controller/api/v1alpha1"
)

// ConfigReloaderReconciler reconciles a ConfigReloader object
type ConfigReloaderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=configreloader.rahmatnugraha.top,resources=configreloaders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=configreloader.rahmatnugraha.top,resources=configreloaders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=configreloader.rahmatnugraha.top,resources=configreloaders/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ConfigReloader object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *ConfigReloaderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the ConfigReloader instance
	var configReloader configreloaderv1alpha1.ConfigReloader
	if err := r.Get(ctx, req.NamespacedName, &configReloader); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch ConfigReloader")
			return ctrl.Result{}, err
		}
		// Object not found (deleted), nothing to do
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling ConfigReloader", "name", configReloader.Name, "namespace", configReloader.Namespace)

	var conditions []metav1.Condition
	needsUpdate := false
	now := metav1.Now()

	// Check ConfigMaps
	configMapHash := ""
	for _, cmName := range configReloader.Spec.ConfigMaps {
		var configMap corev1.ConfigMap
		if err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: configReloader.Namespace}, &configMap); err != nil {
			if client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to fetch ConfigMap", "name", cmName)
				return ctrl.Result{}, err
			}
			conditions = append(conditions, metav1.Condition{
				Type:               "ConfigMapSynced",
				Status:             metav1.ConditionFalse,
				Reason:             "NotFound",
				Message:            fmt.Sprintf("ConfigMap %s not found", cmName),
				LastTransitionTime: now,
			})
			continue
		}
		hash := computeDataHash(configMap.Data)
		configMapHash += hash
		conditions = append(conditions, metav1.Condition{
			Type:               "ConfigMapSynced",
			Status:             metav1.ConditionTrue,
			Reason:             "Synced",
			Message:            fmt.Sprintf("ConfigMap %s is present", cmName),
			LastTransitionTime: now,
		})
	}

	// Check Secrets
	secretHash := ""
	for _, secretName := range configReloader.Spec.Secrets {
		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: configReloader.Namespace}, &secret); err != nil {
			if client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to fetch Secret", "name", secretName)
				return ctrl.Result{}, err
			}
			conditions = append(conditions, metav1.Condition{
				Type:               "SecretSynced",
				Status:             metav1.ConditionFalse,
				Reason:             "NotFound",
				Message:            fmt.Sprintf("Secret %s not found", secretName),
				LastTransitionTime: now,
			})
			continue
		}
		hash := computeDataHash(secret.Data)
		secretHash += hash
		conditions = append(conditions, metav1.Condition{
			Type:               "SecretSynced",
			Status:             metav1.ConditionTrue,
			Reason:             "Synced",
			Message:            fmt.Sprintf("Secret %s is present", secretName),
			LastTransitionTime: now,
		})
	}

	currentHash := configMapHash + secretHash
	lastHash := getAnnotation(configReloader.Annotations, "configreloader.rahmatnugraha.top/last-hash")
	if currentHash != lastHash {
		log.Info("ConfigMap or Secret data changed", "newHash", currentHash, "lastHash", lastHash)
		needsUpdate = true
		// Update the last hash annotation
		if configReloader.Annotations == nil {
			configReloader.Annotations = make(map[string]string)
		}
		configReloader.Annotations["configreloader.rahmatnugraha.top/last-hash"] = currentHash
		if err := r.Update(ctx, &configReloader); err != nil {
			log.Error(err, "failed to update ConfigReloader annotations")
			return ctrl.Result{}, err
		}
	}

	if needsUpdate {
		// Trigger rolling updates for workloads
		for _, workload := range configReloader.Spec.Workloads {
			switch workload.Kind {
			case "Deployment":
				var depl appsv1.Deployment
				if err := r.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: configReloader.Namespace}, &depl); err != nil {
					if client.IgnoreNotFound(err) != nil {
						log.Error(err, "unable to fetch Deployment", "name", workload.Name)
						return ctrl.Result{}, err
					}
					conditions = append(conditions, metav1.Condition{
						Type:               "WorkloadUpdated",
						Status:             metav1.ConditionFalse,
						Reason:             "NotFound",
						Message:            fmt.Sprintf("Deployment %s not found", workload.Name),
						LastTransitionTime: now,
					})
					continue
				}
				if err := r.patchWorkloadAnnotation(ctx, &depl); err != nil {
					log.Error(err, "failed to patch Deployment", "name", workload.Name)
					return ctrl.Result{}, err
				}
				log.Info("Triggered rolling update for Deployment", "name", workload.Name)
				conditions = append(conditions, metav1.Condition{
					Type:               "WorkloadUpdated",
					Status:             metav1.ConditionTrue,
					Reason:             "Updated",
					Message:            fmt.Sprintf("Deployment %s updated", workload.Name),
					LastTransitionTime: now,
				})

			case "StatefulSet":
				var sts appsv1.StatefulSet
				if err := r.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: configReloader.Namespace}, &sts); err != nil {
					if client.IgnoreNotFound(err) != nil {
						log.Error(err, "unable to fetch StatefulSet", "name", workload.Name)
						return ctrl.Result{}, err
					}
					conditions = append(conditions, metav1.Condition{
						Type:               "WorkloadUpdated",
						Status:             metav1.ConditionFalse,
						Reason:             "NotFound",
						Message:            fmt.Sprintf("StatefulSet %s not found", workload.Name),
						LastTransitionTime: now,
					})
					continue
				}
				if err := r.patchWorkloadAnnotation(ctx, &sts); err != nil {
					log.Error(err, "failed to patch StatefulSet", "name", workload.Name)
					return ctrl.Result{}, err
				}
				log.Info("Triggered rolling update for StatefulSet", "name", workload.Name)
				conditions = append(conditions, metav1.Condition{
					Type:               "WorkloadUpdated",
					Status:             metav1.ConditionTrue,
					Reason:             "Updated",
					Message:            fmt.Sprintf("StatefulSet %s updated", workload.Name),
					LastTransitionTime: now,
				})

			case "DaemonSet":
				var ds appsv1.DaemonSet
				if err := r.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: configReloader.Namespace}, &ds); err != nil {
					if client.IgnoreNotFound(err) != nil {
						log.Error(err, "unable to fetch DaemonSet", "name", workload.Name)
						return ctrl.Result{}, err
					}
					conditions = append(conditions, metav1.Condition{
						Type:               "WorkloadUpdated",
						Status:             metav1.ConditionFalse,
						Reason:             "NotFound",
						Message:            fmt.Sprintf("DaemonSet %s not found", workload.Name),
						LastTransitionTime: now,
					})
					continue
				}
				if err := r.patchWorkloadAnnotation(ctx, &ds); err != nil {
					log.Error(err, "failed to patch DaemonSet", "name", workload.Name)
					return ctrl.Result{}, err
				}
				log.Info("Triggered rolling update for DaemonSet", "name", workload.Name)
				conditions = append(conditions, metav1.Condition{
					Type:               "WorkloadUpdated",
					Status:             metav1.ConditionTrue,
					Reason:             "Updated",
					Message:            fmt.Sprintf("DaemonSet %s updated", workload.Name),
					LastTransitionTime: now,
				})

			default:
				log.Info("Unsupported workload kind", "kind", workload.Kind, "name", workload.Name)
			}
		}
	}

	configReloader.Status.ObservedGeneration = configReloader.Generation
	configReloader.Status.LastUpdated = &metav1.Time{Time: time.Now()}
	configReloader.Status.Conditions = conditions
	if err := r.Status().Update(ctx, &configReloader); err != nil {
		log.Error(err, "failed to update ConfigReloader status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ConfigReloaderReconciler) patchWorkloadAnnotation(ctx context.Context, obj client.Object) error {
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))

	// Helper to update template annotations
	updateTemplateAnnotations := func(template *corev1.PodTemplateSpec) {
		if template.ObjectMeta.Annotations == nil {
			template.ObjectMeta.Annotations = make(map[string]string)
		}
		template.ObjectMeta.Annotations["configreloader.rahmatnugraha.top/last-updated"] = time.Now().UTC().Format(time.RFC3339)
	}

	switch workload := obj.(type) {
	case *appsv1.Deployment:
		updateTemplateAnnotations(&workload.Spec.Template)
	case *appsv1.StatefulSet:
		updateTemplateAnnotations(&workload.Spec.Template)
	case *appsv1.DaemonSet:
		updateTemplateAnnotations(&workload.Spec.Template)
	default:
		return fmt.Errorf("unsupported workload type: %T", obj)
	}

	return r.Patch(ctx, obj, patch)
}

func computeDataHash(data interface{}) string {
	hasher := sha256.New()
	switch d := data.(type) {
	case map[string]string: // Handle ConfigMap.Data
		for key, value := range d {
			hasher.Write([]byte(key))
			hasher.Write([]byte(value))
		}
	case map[string][]byte: // Handle Secret.Data
		for key, value := range d {
			hasher.Write([]byte(key))
			hasher.Write(value)
		}
	default:
		return ""
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func getAnnotation(annotations map[string]string, key string) string {
	if annotations == nil {
		return ""
	}
	return annotations[key]
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReloaderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configreloaderv1alpha1.ConfigReloader{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				return r.mapResourceToRequests(mgr.GetClient(), obj)
			}),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				return r.mapResourceToRequests(mgr.GetClient(), obj)
			}),
		).
		Named("configreloader").
		Complete(r)
}

func (r *ConfigReloaderReconciler) mapResourceToRequests(cl client.Client, obj client.Object) []ctrl.Request {
	ctx := context.Background()
	log := log.FromContext(ctx)

	var configReloaders configreloaderv1alpha1.ConfigReloaderList
	if err := cl.List(ctx, &configReloaders); err != nil {
		log.Error(err, "failed to list ConfigReloaders")
		return nil
	}

	requests := []ctrl.Request{}
	objMeta := obj.(metav1.Object)
	for _, cr := range configReloaders.Items {
		for _, cm := range cr.Spec.ConfigMaps {
			if cm == objMeta.GetName() && cr.Namespace == objMeta.GetNamespace() {
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      cr.Name,
						Namespace: cr.Namespace,
					},
				})
				break
			}
		}
		for _, secret := range cr.Spec.Secrets {
			if secret == objMeta.GetName() && cr.Namespace == objMeta.GetNamespace() {
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      cr.Name,
						Namespace: cr.Namespace,
					},
				})
				break
			}
		}
	}
	return requests
}
