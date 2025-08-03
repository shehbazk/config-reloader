package controller

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;update;patch

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
