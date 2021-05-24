package v1alpha1

import (
	"time"

	"github.com/rancher/wrangler/pkg/condition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=lb;lbs,scope=Namespaced
// +kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`
// +kubebuilder:printcolumn:name="InternalAddress",type=string,JSONPath=`.status.internalAddress`
// +kubebuilder:printcolumn:name="ExternalAddress",type=string,JSONPath=`.status.externalAddress`

type LoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LoadBalancerSpec   `json:"spec,omitempty"`
	Status            LoadBalancerStatus `json:"status,omitempty"`
}

type LoadBalancerSpec struct {
	// +optional
	Description string     `json:"description,omitempty"`
	Listeners   []Listener `json:"listeners"`
}

type LoadBalancerStatus struct {
	// +optional
	InternalAddress string `json:"internalAddress,omitempty"`
	// +optional
	ExternalAddress string `json:"externalAddress,omitempty"`
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

type Listener struct {
	Name           string          `json:"name"`
	Port           uint            `json:"port"`
	Protocol       string          `json:"protocol"`
	BackendServers []BackendServer `json:"backendServers"`
	HeathCheck     HeathCheck      `json:"healthCheck,omitempty"`
}

type BackendServer struct {
	Host string `json:"host"`
	Port uint   `json:"port"`
}

type HeathCheck struct {
	Port      int           `json:"port"`
	Threshold int           `json:"thresholdÒ"`
	Interval  time.Duration `json:"interval"`
	Timeout   time.Duration `json:"timeout"`
}

type Condition struct {
	// Type of the condition.
	Type condition.Cond `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

var (
	LoadBalancerReady condition.Cond = "Ready"
)
