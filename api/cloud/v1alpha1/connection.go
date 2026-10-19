package v1alpha1

import (
	tcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConnectionSpec describes the desired configuration of a Connection to a
// Temporal Cloud API server. It describes where credentials can be found for
// instantiating a Temporal Cloud client object to that Temporal Cloud API
// server.
type ConnectionSpec struct {
	// HostPort is the host:port of the Temporal Cloud API server endpoint to
	// connect to.
	//
	// Defaults to "saas-api.tmprl.cloud:443"
	//
	// This is a gRPC address and therefore can also support a
	// special-formatted address of "<resolver>:///<value>" that will use a
	// registered resolver. By default all hosts returned from the resolver
	// will be used in a round-robin fashion.
	//
	// The "dns" resolver is registered by and used by default.
	// +optional
	HostPort string `json:"hostPort,omitempty"`
	// TLS configures TLS behaviour for the Temporal Cloud server connection.
	// +optional
	TLS tcorev1.ServerTLSConfig `json:"tls,omitempty"`
	// Credentials contains desired configuration for the Temporal Cloud client
	// credentials.
	Credentials CredentialsConfig `json:"credentials"`
}

// ConnectionStatus defines the observed state of a Connection
type ConnectionStatus struct {
	tcorev1.StatusBase `json:",inline"`
}

//+kubebuilder:object:root=true
//+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.hostPort",description="Temporal API server host:port"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Age"

// Connection describes a client Connection to a Temporal Cloud API server.
type Connection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConnectionSpec   `json:"spec,omitempty"`
	Status ConnectionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConnectionList contains a list of Connection
type ConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Connection `json:"items"`
}
