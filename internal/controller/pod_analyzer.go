package controller

import (
	corev1 "k8s.io/api/core/v1"
)

func (r *ConfigReloaderReconciler) podUsesWatchedResources(
	pod *corev1.Pod,
	watchedCMs, watchedSecrets map[string]bool,
) bool {
	// Check volumes
	if r.podVolumesUseWatchedResources(pod, watchedCMs, watchedSecrets) {
		return true
	}

	if r.containersUseWatchedResources(pod.Spec.Containers, pod.Namespace, watchedCMs, watchedSecrets) {
		return true
	}

	if r.containersUseWatchedResources(pod.Spec.InitContainers, pod.Namespace, watchedCMs, watchedSecrets) {
		return true
	}

	return false
}

func (r *ConfigReloaderReconciler) podVolumesUseWatchedResources(
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
	return false
}

func (r *ConfigReloaderReconciler) containersUseWatchedResources(
	containers []corev1.Container,
	namespace string,
	watchedCMs, watchedSecrets map[string]bool,
) bool {
	for _, container := range containers {
		// Check envFrom
		if r.containerEnvFromUseWatchedResources(container, namespace, watchedCMs, watchedSecrets) {
			return true
		}

		// Check individual env vars
		if r.containerEnvVarsUseWatchedResources(container, namespace, watchedCMs, watchedSecrets) {
			return true
		}
	}
	return false
}

func (r *ConfigReloaderReconciler) containerEnvFromUseWatchedResources(
	container corev1.Container,
	namespace string,
	watchedCMs, watchedSecrets map[string]bool,
) bool {
	for _, envFrom := range container.EnvFrom {
		if envFrom.ConfigMapRef != nil {
			key := namespace + "/" + envFrom.ConfigMapRef.Name
			if watchedCMs[key] {
				return true
			}
		}
		if envFrom.SecretRef != nil {
			key := namespace + "/" + envFrom.SecretRef.Name
			if watchedSecrets[key] {
				return true
			}
		}
	}
	return false
}

func (r *ConfigReloaderReconciler) containerEnvVarsUseWatchedResources(
	container corev1.Container,
	namespace string,
	watchedCMs, watchedSecrets map[string]bool,
) bool {
	for _, env := range container.Env {
		if env.ValueFrom != nil {
			if env.ValueFrom.ConfigMapKeyRef != nil {
				key := namespace + "/" + env.ValueFrom.ConfigMapKeyRef.Name
				if watchedCMs[key] {
					return true
				}
			}
			if env.ValueFrom.SecretKeyRef != nil {
				key := namespace + "/" + env.ValueFrom.SecretKeyRef.Name
				if watchedSecrets[key] {
					return true
				}
			}
		}
	}
	return false
}
