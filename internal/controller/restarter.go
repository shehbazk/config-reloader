package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	configv1 "github.com/shehbazk/config-reloader-operator/api/v1"
)

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
	restartAnnotation := fmt.Sprintf("config.dev/restarted-at-%d", now.Unix())

	for _, pod := range podList.Items {
		// Check if pod uses watched resources
		if !r.podUsesWatchedResources(&pod, watchedCMs, watchedSecrets) {
			continue
		}

		logger.Info("Processing pod for restart", "pod", pod.Name, "namespace", pod.Namespace)

		switch cr.Spec.RestartPolicy {
		case configv1.RestartPolicyAnnotation:
			restartInfo := r.handleAnnotationRestart(ctx, &pod, restartAnnotation, now)
			if restartInfo != nil {
				restartedPods = append(restartedPods, *restartInfo)
			}

		case configv1.RestartPolicyDelete:
			restartInfo := r.handleDeleteRestart(ctx, &pod, now)
			if restartInfo != nil {
				restartedPods = append(restartedPods, *restartInfo)
			}
		}
	}

	return restartedPods, nil
}

// handleAnnotationRestart handles restart via annotation updates
func (r *ConfigReloaderReconciler) handleAnnotationRestart(
	ctx context.Context,
	pod *corev1.Pod,
	restartAnnotation string,
	now metav1.Time,
) *configv1.PodRestart {
	logger := log.FromContext(ctx)

	// Handle controller-managed pods
	if len(pod.OwnerReferences) > 0 {
		restarted, err := r.restartControllerManagedPod(ctx, pod, restartAnnotation)
		if err != nil {
			logger.Error(err, "failed to restart controller-managed pod", "pod", pod.Name)
			return nil
		}
		if restarted {
			return &configv1.PodRestart{
				PodName:     pod.Name,
				Namespace:   pod.Namespace,
				RestartTime: &now,
				Reason:      "ConfigMap/Secret changed - controller updated",
			}
		}
	} else {
		logger.Info("Standalone pod detected with annotation restart policy - this won't restart the pod",
			"pod", pod.Name, "suggestion", "use delete restart policy for standalone pods")

		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[ReloadAnnotation] = now.Format(time.RFC3339)

		if err := r.Update(ctx, pod); err != nil {
			logger.Error(err, "failed to update pod annotation", "pod", pod.Name)
			return nil
		}

		return &configv1.PodRestart{
			PodName:     pod.Name,
			Namespace:   pod.Namespace,
			RestartTime: &now,
			Reason:      "ConfigMap/Secret changed - annotation updated (pod not restarted)",
		}
	}

	return nil
}

func (r *ConfigReloaderReconciler) handleDeleteRestart(
	ctx context.Context,
	pod *corev1.Pod,
	now metav1.Time,
) *configv1.PodRestart {
	logger := log.FromContext(ctx)

	logger.Info("Deleting pod for restart", "pod", pod.Name, "namespace", pod.Namespace)

	if err := r.Delete(ctx, pod); err != nil {
		logger.Error(err, "failed to delete pod", "pod", pod.Name)
		return nil
	}

	return &configv1.PodRestart{
		PodName:     pod.Name,
		Namespace:   pod.Namespace,
		RestartTime: &now,
		Reason:      "ConfigMap/Secret changed - pod deleted",
	}
}

func (r *ConfigReloaderReconciler) restartControllerManagedPod(
	ctx context.Context,
	pod *corev1.Pod,
	restartAnnotation string,
) (bool, error) {
	logger := log.FromContext(ctx)

	for _, ownerRef := range pod.OwnerReferences {
		switch ownerRef.Kind {
		case "Deployment":
			return r.restartDeployment(ctx, pod.Namespace, ownerRef.Name, restartAnnotation)
		case "StatefulSet":
			return r.restartStatefulSet(ctx, pod.Namespace, ownerRef.Name, restartAnnotation)
		case "DaemonSet":
			return r.restartDaemonSet(ctx, pod.Namespace, ownerRef.Name, restartAnnotation)
		case "ReplicaSet":
			return r.restartReplicaSet(ctx, pod.Namespace, ownerRef.Name, restartAnnotation)
		default:
			logger.Info("Unsupported controller type for annotation restart",
				"kind", ownerRef.Kind, "name", ownerRef.Name)
		}
	}

	return false, nil
}
