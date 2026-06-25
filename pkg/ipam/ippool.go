package ipam

import (
	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

func IsGlobalIPPool(pool *lbv1.IPPool) bool {
	return len(pool.Spec.Selector.Scope) == 1 && pool.Spec.Selector.Scope[0].Namespace == All &&
		pool.Spec.Selector.Scope[0].Project == All && pool.Spec.Selector.Scope[0].GuestCluster == All
}

// a global pool of a specific network
// if the network is All (could only be passed for VM type LB), then it just checks the IsGlobalIPPool
func IsGlobalIPPoolOfNetwork(pool *lbv1.IPPool, network string) bool {
	return (network == All || pool.Spec.Selector.Network == network) && IsGlobalIPPool(pool)
}
