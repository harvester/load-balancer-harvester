package ipam

import (
	"k8s.io/apimachinery/pkg/labels"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
)

const All = "*"

type Selector struct {
	ctllbv1.IPPoolCache
}

type Matcher lbv1.Selector

type Requirement struct {
	Network   string
	Project   string
	Namespace string
	Cluster   string
}

func NewSelector(cache ctllbv1.IPPoolCache) *Selector {
	return &Selector{
		IPPoolCache: cache,
	}
}

func NewMatcher(poolSelector lbv1.Selector) *Matcher {
	m := (Matcher)(poolSelector)
	return &m
}

func (m *Matcher) Matches(r *Requirement) bool {
	if m.Network != r.Network {
		return false
	}

	for _, tuple := range m.Scope {
		if isMatch(&tuple, r) {
			return true
		}
	}

	return false
}

func (s *Selector) Select(r *Requirement) (*lbv1.IPPool, error) {
	pools, err := s.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	ipPool := &lbv1.IPPool{}
	var priority uint32
	for _, pool := range pools {
		// If the priority is not zero, every pool has different priority value.
		if NewMatcher(pool.Spec.Selector).Matches(r) && pool.Spec.Selector.Priority >= priority {
			ipPool = pool
			priority = pool.Spec.Selector.Priority
		}
	}

	return ipPool, nil
}

func isMatch(t *lbv1.Tuple, r *Requirement) bool {
	// if the value of requirement is *, we think it matches any value.
	if (t.Project == All || r.Project == All || t.Project == r.Project) &&
		(t.Namespace == All || r.Namespace == All || t.Namespace == r.Namespace) &&
		(t.GuestCluster == All || r.Cluster == All || t.GuestCluster == r.Cluster) {
		return true
	}

	return false
}
