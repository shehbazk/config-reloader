package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigReloaderSpec defines the desired state of ConfigReloader
type ConfigReloaderSpec struct {
	// ConfigMaps to watch for changes
	// +optional
	ConfigMaps []ResourceRef `json:"configMaps,omitempty"`

	// Secrets to watch for changes
	// +optional
	Secrets []ResourceRef `json:"secrets,omitempty"`

	// Selector for pods to restart when config changes
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// RestartPolicy defines how to restart pods
	// +kubebuilder:validation:Enum=annotation;delete
	// +kubebuilder:default=annotation
	RestartPolicy RestartPolicy `json:"restartPolicy,omitempty"`

	// IgnoreOwnerReferences ignores pods that are owned by controllers
	// +kubebuilder:default=false
	IgnoreOwnerReferences bool `json:"ignoreOwnerReferences,omitempty"`
}

// ResourceRef references a ConfigMap or Secret
type ResourceRef struct {
	// Name of the resource
	Name string `json:"name"`

	// Namespace of the resource (defaults to same namespace as ConfigReloader)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// RestartPolicy defines restart strategies
// +kubebuilder:validation:Enum=annotation;delete
type RestartPolicy string

const (
	// RestartPolicyAnnotation restarts pods by updating annotations
	RestartPolicyAnnotation RestartPolicy = "annotation"
	// RestartPolicyDelete restarts pods by deleting them
	RestartPolicyDelete RestartPolicy = "delete"
)

// ConfigReloaderStatus defines the observed state of ConfigReloader
type ConfigReloaderStatus struct {
	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastReloadTime indicates when the last reload occurred
	// +optional
	LastReloadTime *metav1.Time `json:"lastReloadTime,omitempty"`

	// WatchedResources shows currently watched ConfigMaps and Secrets
	// +optional
	WatchedResources []WatchedResource `json:"watchedResources,omitempty"`

	// PodsRestarted tracks recently restarted pods
	// +optional
	PodsRestarted []PodRestart `json:"podsRestarted,omitempty"`
}

// WatchedResource represents a resource being watched
type WatchedResource struct {
	// Kind of resource (ConfigMap or Secret)
	Kind string `json:"kind"`
	// Name of the resource
	Name string `json:"name"`
	// Namespace of the resource
	Namespace string `json:"namespace"`
	// ResourceVersion of the last seen version
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// LastUpdateTime when this resource was last updated
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// PodRestart tracks a pod restart event
type PodRestart struct {
	// PodName that was restarted
	PodName string `json:"podName"`
	// Namespace of the pod
	Namespace string `json:"namespace"`
	// RestartTime when the restart occurred
	RestartTime *metav1.Time `json:"restartTime"`
	// Reason for the restart
	Reason string `json:"reason"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cr
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Last Reload",type="string",JSONPath=".status.lastReloadTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ConfigReloader is the Schema for the configreloaders API
type ConfigReloader struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigReloaderSpec   `json:"spec,omitempty"`
	Status ConfigReloaderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigReloaderList contains a list of ConfigReloader
type ConfigReloaderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigReloader `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigReloader{}, &ConfigReloaderList{})
}
