package lb

import lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"

type Manager interface {
	EnsureLoadBalancer(lb *lbv1.LoadBalancer) error
	DeleteLoadBalancer(lb *lbv1.LoadBalancer)
}
