package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configv1 "github.com/shehbazk/config-reloader-operator/api/v1"
)

type ConfigMapSecretHandler struct {
	Client client.Client
}

func (h *ConfigMapSecretHandler) Create(ctx context.Context, evt event.TypedCreateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.enqueueConfigReloaders(ctx, evt.Object, q)
}

func (h *ConfigMapSecretHandler) Update(ctx context.Context, evt event.TypedUpdateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.enqueueConfigReloaders(ctx, evt.ObjectNew, q)
}

func (h *ConfigMapSecretHandler) Delete(ctx context.Context, evt event.TypedDeleteEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.enqueueConfigReloaders(ctx, evt.Object, q)
}

func (h *ConfigMapSecretHandler) Generic(ctx context.Context, evt event.TypedGenericEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.enqueueConfigReloaders(ctx, evt.Object, q)
}

func (h *ConfigMapSecretHandler) enqueueConfigReloaders(ctx context.Context, obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	logger := log.FromContext(ctx)

	var configReloaderList configv1.ConfigReloaderList
	if err := h.Client.List(ctx, &configReloaderList); err != nil {
		logger.Error(err, "failed to list ConfigReloaders")
		return
	}

	resourceName := obj.GetName()
	resourceNamespace := obj.GetNamespace()
	var resourceKind string

	switch obj.(type) {
	case *corev1.ConfigMap:
		resourceKind = "ConfigMap"
	case *corev1.Secret:
		resourceKind = "Secret"
	default:
		return
	}

	for _, cr := range configReloaderList.Items {
		if h.configReloaderWatchesResource(&cr, resourceKind, resourceName, resourceNamespace) {
			logger.Info("Enqueueing ConfigReloader due to resource change",
				"configReloader", cr.Name,
				"namespace", cr.Namespace,
				"resourceKind", resourceKind,
				"resourceName", resourceName,
				"resourceNamespace", resourceNamespace)

			q.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cr.Name,
					Namespace: cr.Namespace,
				},
			})
		}
	}
}

func (h *ConfigMapSecretHandler) configReloaderWatchesResource(cr *configv1.ConfigReloader, resourceKind, resourceName, resourceNamespace string) bool {
	if resourceKind == "ConfigMap" {
		for _, cmRef := range cr.Spec.ConfigMaps {
			namespace := cmRef.Namespace
			if namespace == "" {
				namespace = cr.Namespace
			}
			if cmRef.Name == resourceName && namespace == resourceNamespace {
				return true
			}
		}
	}

	if resourceKind == "Secret" {
		for _, secretRef := range cr.Spec.Secrets {
			namespace := secretRef.Namespace
			if namespace == "" {
				namespace = cr.Namespace
			}
			if secretRef.Name == resourceName && namespace == resourceNamespace {
				return true
			}
		}
	}

	return false
}
