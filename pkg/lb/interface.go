package lb

import (
	"k8s.io/apimachinery/pkg/types"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

type Manager interface {
	EnsureLoadBalancer(lb *lbv1.LoadBalancer) error
	DeleteLoadBalancer(lb *lbv1.LoadBalancer) error
	GetBackendServers(lb *lbv1.LoadBalancer) ([]BackendServer, error)
	// AddBackendServers returns true if backend servers are really added the LB, otherwise returns false
	AddBackendServers(lb *lbv1.LoadBalancer, servers []BackendServer) (bool, error)
	// RemoveBackendServers returns true if backend servers are really removed from the LB, otherwise returns false
	RemoveBackendServers(lb *lbv1.LoadBalancer, servers []BackendServer) (bool, error)
}

type BackendServer interface {
	GetUID() types.UID
	GetNamespace() string
	GetName() string
	GetAddress() (string, bool)
}
