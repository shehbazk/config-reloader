package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	configv1 "github.com/shehbazk/config-reloader-operator/api/v1"
)

const (
	ConfigReloaderFinalizer = "config.dev/finalizer"
	ReloadAnnotation        = "config.dev/last-reload"
)

// ConfigReloaderReconciler reconciles a ConfigReloader object
type ConfigReloaderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=config.dev,resources=configreloaders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.dev,resources=configreloaders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=config.dev,resources=configreloaders/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch;delete

func (r *ConfigReloaderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ConfigReloader instance
	var configReloader configv1.ConfigReloader
	if err := r.Get(ctx, req.NamespacedName, &configReloader); err != nil {
		logger.Error(err, "unable to fetch ConfigReloader")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if configReloader.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &configReloader)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&configReloader, ConfigReloaderFinalizer) {
		controllerutil.AddFinalizer(&configReloader, ConfigReloaderFinalizer)
		return ctrl.Result{}, r.Update(ctx, &configReloader)
	}

	return r.reconcileConfigReloader(ctx, &configReloader)
}

func (r *ConfigReloaderReconciler) reconcileConfigReloader(
	ctx context.Context,
	cr *configv1.ConfigReloader,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	hasChanges, err := r.checkForChanges(ctx, cr)
	if err != nil {
		r.updateCondition(cr, "Ready", metav1.ConditionFalse, "CheckFailed", err.Error())
		return ctrl.Result{RequeueAfter: time.Minute * 5}, r.Status().Update(ctx, cr)
	}

	if hasChanges {
		logger.Info("Detected changes in watched resources, restarting pods")

		restarted, err := r.restartAffectedPods(ctx, cr)
		if err != nil {
			r.updateCondition(cr, "Ready", metav1.ConditionFalse, "RestartFailed", err.Error())
			return ctrl.Result{RequeueAfter: time.Minute * 5}, r.Status().Update(ctx, cr)
		}

		now := metav1.Now()
		cr.Status.LastReloadTime = &now
		cr.Status.PodsRestarted = append(cr.Status.PodsRestarted, restarted...)

		if len(cr.Status.PodsRestarted) > 10 {
			cr.Status.PodsRestarted = cr.Status.PodsRestarted[len(cr.Status.PodsRestarted)-10:]
		}
	}

	r.updateWatchedResourcesStatus(ctx, cr)

	r.updateCondition(cr, "Ready", metav1.ConditionTrue, "ReconcileSuccess", "ConfigReloader is ready")

	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue to check for changes periodically
	return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
}

func (r *ConfigReloaderReconciler) checkForChanges(ctx context.Context, cr *configv1.ConfigReloader) (bool, error) {
	hasChanges := false

	for _, cmRef := range cr.Spec.ConfigMaps {
		namespace := cmRef.Namespace
		if namespace == "" {
			namespace = cr.Namespace
		}

		var cm corev1.ConfigMap
		if err := r.Get(ctx, types.NamespacedName{Name: cmRef.Name, Namespace: namespace}, &cm); err != nil {
			return false, fmt.Errorf("failed to get ConfigMap %s/%s: %w", namespace, cmRef.Name, err)
		}

		if r.hasResourceChanged(cr, "ConfigMap", cmRef.Name, namespace, cm.ResourceVersion) {
			hasChanges = true
		}
	}

	for _, secretRef := range cr.Spec.Secrets {
		namespace := secretRef.Namespace
		if namespace == "" {
			namespace = cr.Namespace
		}

		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Name: secretRef.Name, Namespace: namespace}, &secret); err != nil {
			return false, fmt.Errorf("failed to get Secret %s/%s: %w", namespace, secretRef.Name, err)
		}

		if r.hasResourceChanged(cr, "Secret", secretRef.Name, namespace, secret.ResourceVersion) {
			hasChanges = true
		}
	}

	return hasChanges, nil
}

func (r *ConfigReloaderReconciler) hasResourceChanged(
	cr *configv1.ConfigReloader,
	kind, name, namespace, resourceVersion string,
) bool {
	for _, watched := range cr.Status.WatchedResources {
		if watched.Kind == kind && watched.Name == name && watched.Namespace == namespace {
			return watched.ResourceVersion != resourceVersion
		}
	}
	// First time seeing this resource, so it's "changed" (new)
	return true
}
func (r *ConfigReloaderReconciler) restartAffectedPods(ctx context.Context,
	cr *configv1.ConfigReloader) ([]configv1.PodRestart, error) {
	logger := log.FromContext(ctx)

	restartedPods := make([]configv1.PodRestart, 0, 10)

	watchedCMs, watchedSecrets := r.buildWatchedResourcesMaps(cr)

	var podList corev1.PodList
	listOpts := []client.ListOption{
		client.InNamespace(cr.Namespace),
	}

	if cr.Spec.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(cr.Spec.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid selector: %w", err)
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: selector})
	}

	if err := r.List(ctx, &podList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	now := metav1.Now()

	for _, pod := range podList.Items {
		// Check if pod uses watched resources
		if !r.podUsesWatchedResources(&pod, watchedCMs, watchedSecrets) {
			continue
		}

		logger.Info("Deleting pod for restart", "pod", pod.Name, "namespace", pod.Namespace)

		if err := r.Delete(ctx, &pod); err != nil {
			logger.Error(err, "failed to delete pod", "pod", pod.Name)
			continue
		}

		restartedPods = append(restartedPods, configv1.PodRestart{
			PodName:     pod.Name,
			Namespace:   pod.Namespace,
			RestartTime: &now,
			Reason:      "ConfigMap/Secret changed - pod deleted",
		})
	}

	return restartedPods, nil
}

func (r *ConfigReloaderReconciler) buildWatchedResourcesMaps(
	cr *configv1.ConfigReloader,
) (map[string]bool, map[string]bool) {
	watchedCMs := make(map[string]bool)
	watchedSecrets := make(map[string]bool)

	for _, cm := range cr.Spec.ConfigMaps {
		namespace := cm.Namespace
		if namespace == "" {
			namespace = cr.Namespace
		}
		watchedCMs[namespace+"/"+cm.Name] = true
	}

	for _, secret := range cr.Spec.Secrets {
		namespace := secret.Namespace
		if namespace == "" {
			namespace = cr.Namespace
		}
		watchedSecrets[namespace+"/"+secret.Name] = true
	}

	return watchedCMs, watchedSecrets
}

func (r *ConfigReloaderReconciler) podUsesWatchedResources(
	pod *corev1.Pod,
	watchedCMs, watchedSecrets map[string]bool,
) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil {
			key := pod.Namespace + "/" + volume.ConfigMap.Name
			if watchedCMs[key] {
				return true
			}
		}
		if volume.Secret != nil {
			key := pod.Namespace + "/" + volume.Secret.SecretName
			if watchedSecrets[key] {
				return true
			}
		}
	}

	for _, container := range pod.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				key := pod.Namespace + "/" + envFrom.ConfigMapRef.Name
				if watchedCMs[key] {
					return true
				}
			}
			if envFrom.SecretRef != nil {
				key := pod.Namespace + "/" + envFrom.SecretRef.Name
				if watchedSecrets[key] {
					return true
				}
			}
		}

		for _, env := range container.Env {
			if env.ValueFrom != nil {
				if env.ValueFrom.ConfigMapKeyRef != nil {
					key := pod.Namespace + "/" + env.ValueFrom.ConfigMapKeyRef.Name
					if watchedCMs[key] {
						return true
					}
				}
				if env.ValueFrom.SecretKeyRef != nil {
					key := pod.Namespace + "/" + env.ValueFrom.SecretKeyRef.Name
					if watchedSecrets[key] {
						return true
					}
				}
			}
		}
	}

	for _, container := range pod.Spec.InitContainers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				key := pod.Namespace + "/" + envFrom.ConfigMapRef.Name
				if watchedCMs[key] {
					return true
				}
			}
			if envFrom.SecretRef != nil {
				key := pod.Namespace + "/" + envFrom.SecretRef.Name
				if watchedSecrets[key] {
					return true
				}
			}
		}

		for _, env := range container.Env {
			if env.ValueFrom != nil {
				if env.ValueFrom.ConfigMapKeyRef != nil {
					key := pod.Namespace + "/" + env.ValueFrom.ConfigMapKeyRef.Name
					if watchedCMs[key] {
						return true
					}
				}
				if env.ValueFrom.SecretKeyRef != nil {
					key := pod.Namespace + "/" + env.ValueFrom.SecretKeyRef.Name
					if watchedSecrets[key] {
						return true
					}
				}
			}
		}
	}

	return false
}

func (r *ConfigReloaderReconciler) updateWatchedResourcesStatus(
	ctx context.Context,
	cr *configv1.ConfigReloader,
) {
	// Pre-allocate slice with known capacity
	totalResources := len(cr.Spec.ConfigMaps) + len(cr.Spec.Secrets)
	watchedResources := make([]configv1.WatchedResource, 0, totalResources)
	now := metav1.Now()

	for _, cmRef := range cr.Spec.ConfigMaps {
		namespace := cmRef.Namespace
		if namespace == "" {
			namespace = cr.Namespace
		}

		var cm corev1.ConfigMap
		if err := r.Get(ctx, types.NamespacedName{Name: cmRef.Name, Namespace: namespace}, &cm); err != nil {
			// Resource not found, skip it
			continue
		}

		watchedResources = append(watchedResources, configv1.WatchedResource{
			Kind:            "ConfigMap",
			Name:            cmRef.Name,
			Namespace:       namespace,
			ResourceVersion: cm.ResourceVersion,
			LastUpdateTime:  &now,
		})
	}

	for _, secretRef := range cr.Spec.Secrets {
		namespace := secretRef.Namespace
		if namespace == "" {
			namespace = cr.Namespace
		}

		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Name: secretRef.Name, Namespace: namespace}, &secret); err != nil {
			continue
		}

		watchedResources = append(watchedResources, configv1.WatchedResource{
			Kind:            "Secret",
			Name:            secretRef.Name,
			Namespace:       namespace,
			ResourceVersion: secret.ResourceVersion,
			LastUpdateTime:  &now,
		})
	}

	cr.Status.WatchedResources = watchedResources
}

func (r *ConfigReloaderReconciler) updateCondition(cr *configv1.ConfigReloader,
	conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	for i, existingCondition := range cr.Status.Conditions {
		if existingCondition.Type == conditionType {
			if existingCondition.Status != status {
				cr.Status.Conditions[i] = condition
			}
			return
		}
	}

	cr.Status.Conditions = append(cr.Status.Conditions, condition)
}

func (r *ConfigReloaderReconciler) handleDeletion(
	ctx context.Context,
	cr *configv1.ConfigReloader,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling ConfigReloader deletion")

	controllerutil.RemoveFinalizer(cr, ConfigReloaderFinalizer)
	return ctrl.Result{}, r.Update(ctx, cr)
}

func (r *ConfigReloaderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1.ConfigReloader{}).
		Complete(r)
}
