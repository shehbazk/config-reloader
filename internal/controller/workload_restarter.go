package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ConfigReloaderReconciler) restartDeployment(
	ctx context.Context,
	namespace, name, restartAnnotation string,
) (bool, error) {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployment); err != nil {
		return false, fmt.Errorf("failed to get Deployment %s/%s: %w", namespace, name, err)
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}

	deployment.Spec.Template.Annotations[restartAnnotation] = metav1.Now().Format("2006-01-02T15:04:05Z")

	if err := r.Update(ctx, deployment); err != nil {
		return false, fmt.Errorf("failed to update Deployment %s/%s: %w", namespace, name, err)
	}

	return true, nil
}

func (r *ConfigReloaderReconciler) restartStatefulSet(
	ctx context.Context,
	namespace, name, restartAnnotation string,
) (bool, error) {
	statefulSet := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, statefulSet); err != nil {
		return false, fmt.Errorf("failed to get StatefulSet %s/%s: %w", namespace, name, err)
	}

	if statefulSet.Spec.Template.Annotations == nil {
		statefulSet.Spec.Template.Annotations = make(map[string]string)
	}

	statefulSet.Spec.Template.Annotations[restartAnnotation] = metav1.Now().Format("2006-01-02T15:04:05Z")

	if err := r.Update(ctx, statefulSet); err != nil {
		return false, fmt.Errorf("failed to update StatefulSet %s/%s: %w", namespace, name, err)
	}

	return true, nil
}

func (r *ConfigReloaderReconciler) restartDaemonSet(
	ctx context.Context,
	namespace, name, restartAnnotation string,
) (bool, error) {
	daemonSet := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, daemonSet); err != nil {
		return false, fmt.Errorf("failed to get DaemonSet %s/%s: %w", namespace, name, err)
	}

	if daemonSet.Spec.Template.Annotations == nil {
		daemonSet.Spec.Template.Annotations = make(map[string]string)
	}

	daemonSet.Spec.Template.Annotations[restartAnnotation] = metav1.Now().Format("2006-01-02T15:04:05Z")

	if err := r.Update(ctx, daemonSet); err != nil {
		return false, fmt.Errorf("failed to update DaemonSet %s/%s: %w", namespace, name, err)
	}

	return true, nil
}

func (r *ConfigReloaderReconciler) restartReplicaSet(
	ctx context.Context,
	namespace, name, restartAnnotation string,
) (bool, error) {
	replicaSet := &appsv1.ReplicaSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, replicaSet); err != nil {
		return false, fmt.Errorf("failed to get ReplicaSet %s/%s: %w", namespace, name, err)
	}

	for _, ownerRef := range replicaSet.OwnerReferences {
		if ownerRef.Kind == "Deployment" {
			return r.restartDeployment(ctx, namespace, ownerRef.Name, restartAnnotation)
		}
	}

	if replicaSet.Spec.Template.Annotations == nil {
		replicaSet.Spec.Template.Annotations = make(map[string]string)
	}

	replicaSet.Spec.Template.Annotations[restartAnnotation] = metav1.Now().Format("2006-01-02T15:04:05Z")

	if err := r.Update(ctx, replicaSet); err != nil {
		return false, fmt.Errorf("failed to update ReplicaSet %s/%s: %w", namespace, name, err)
	}

	return true, nil
}
