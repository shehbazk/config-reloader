package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/shehbazk/config-reloader-operator/api/v1"
)

func (r *ConfigReloaderReconciler) updateCondition(
	cr *configv1.ConfigReloader,
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
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
