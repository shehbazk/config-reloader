package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	configv1 "github.com/shehbazk/config-reloader-operator/api/v1"
)

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

func (r *ConfigReloaderReconciler) updateWatchedResourcesStatus(
	ctx context.Context,
	cr *configv1.ConfigReloader,
) {
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

// Map for quick lookup of watched resources
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
