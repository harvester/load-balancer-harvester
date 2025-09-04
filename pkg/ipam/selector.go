package ipam

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
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

	for i := range m.Scope {
		if isMatch(&m.Scope[i], r) {
			return true
		}
	}

	return false
}

func (m *Matcher) MatchesAllowEmpty(r *Requirement) bool {
	// network must be strictly matched
	if m.Network != r.Network {
		return false
	}

	// if none of the scope is defined (all empty) allows match all
	if len(m.Scope) == 0 {
		return true
	}

	for i := range m.Scope {
		if isMatchAllowEmpty(&m.Scope[i], r) {
			return true
		}
	}

	return false
}

// Select returns the pool that matches the requirement.
// Priority:
// 1. the pool that matches the requirement and has the highest priority
// 2. the global pool
func (s *Selector) Select(r *Requirement) (*lbv1.IPPool, error) {
	if r == nil {
		return nil, fmt.Errorf("the requirement to select a pool can't be empty")
	}
	pools, err := s.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var selectedPool, globalPool *lbv1.IPPool
	var priority uint32
	for _, pool := range pools {
		if pool.Labels != nil && pool.Labels[utils.KeyGlobalIPPool] == utils.ValueTrue {
			globalPool = pool
			continue
		}
		// If the priority is not zero, every pool has different priority value.
		if NewMatcher(pool.Spec.Selector).Matches(r) && pool.Spec.Selector.Priority >= priority {
			selectedPool = pool
			priority = pool.Spec.Selector.Priority
		}
	}
	// If there is no pool matches the requirement, we will use the global pool if existing.
	if selectedPool != nil {
		return selectedPool, nil
	}
	if globalPool != nil {
		return globalPool, nil
	}

	// if this is not a cluster type request, return anyway
	if r.Cluster == "" {
		return globalPool, nil
	}

	// for cluster type request, allow another try
	// bacause the guest cluster side has no UI option to set Project, Namespace, GuestCluster
	return s.SelectAllowEmpty(r)
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

func (s *Selector) SelectAllowEmpty(r *Requirement) (*lbv1.IPPool, error) {
	pools, err := s.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var selectedPool, globalPool *lbv1.IPPool
	var priority uint32
	for _, pool := range pools {
		if pool.Labels != nil && pool.Labels[utils.KeyGlobalIPPool] == utils.ValueTrue {
			globalPool = pool
			continue
		}
		// If the priority is not zero, every pool has different priority value.
		if NewMatcher(pool.Spec.Selector).MatchesAllowEmpty(r) && pool.Spec.Selector.Priority >= priority {
			selectedPool = pool
			priority = pool.Spec.Selector.Priority
		}
	}
	// If there is no pool matches the requirement, we will use the global pool if existing.
	if selectedPool != nil {
		return selectedPool, nil
	}
	return globalPool, nil
}

func isMatchAllowEmpty(t *lbv1.Tuple, r *Requirement) bool {
	// by default, Harvester UI only supports to set Scope of Namespace, and it allows to be empty
	// and it has no option to set Project and GuestCluster
	// user can only set them via kubectl or `Edit Yaml`
	if t.Project == "" && t.Namespace == "" && t.GuestCluster == "" {
		return true
	}

	return isMatch(t, r)
}
