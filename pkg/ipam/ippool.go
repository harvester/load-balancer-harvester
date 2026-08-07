package ipam

import (
	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

func IsGlobalIPPool(pool *lbv1.IPPool) bool {
	return len(pool.Spec.Selector.Scope) == 1 && pool.Spec.Selector.Scope[0].Namespace == All &&
		pool.Spec.Selector.Scope[0].Project == All && pool.Spec.Selector.Scope[0].GuestCluster == All
}

// a global pool of a specific network
func IsGlobalIPPoolOfNetwork(pool *lbv1.IPPool, network string) bool {
	return pool.Spec.Selector.Network == network && IsGlobalIPPool(pool)
}
