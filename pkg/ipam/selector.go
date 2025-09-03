package ipam

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const All = "*"
const EmptySelector = ""

type Selector struct {
	ctllbv1.IPPoolCache
}

type Matcher struct {
	selector  lbv1.Selector
	looseMode bool
}

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
	return &Matcher{
		selector:  poolSelector,
		looseMode: false,
	}
}

func NewMatcherWithMode(poolSelector lbv1.Selector, looseMode bool) *Matcher {
	return &Matcher{
		selector:  poolSelector,
		looseMode: looseMode,
	}
}

func (m *Matcher) Matches(r *Requirement) bool {
	if m.selector.Network != r.Network {
		return false
	}

	for i := range m.selector.Scope {
		if isMatch(&m.selector.Scope[i], r, m.looseMode) {
			return true
		}
	}

	return false
}

// Select returns the pool that matches the requirement.
// Priority:
// 1. the pool that matches the requirement and has the highest priority
// 2. the global pool
func (s *Selector) Select(r *Requirement, looseMode bool) (*lbv1.IPPool, error) {
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
		if NewMatcherWithMode(pool.Spec.Selector, looseMode).Matches(r) && pool.Spec.Selector.Priority >= priority {
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

func isMatch(t *lbv1.Tuple, r *Requirement, looseMode bool) bool {
	// by default, the strict match is used
	if !looseMode {
		// if the value of requirement is *, we think it matches any value.
		if (t.Project == All || r.Project == All || t.Project == r.Project) &&
			(t.Namespace == All || r.Namespace == All || t.Namespace == r.Namespace) &&
			(t.GuestCluster == All || r.Cluster == All || t.GuestCluster == r.Cluster) {
			return true
		}

		return false
	}

	// is loose mode, pool scope `Project` and `GuestCluster` are processed specially
	// the EmptySelector ("") means matching all
	// as they are not exposed on UI for user to set
	if (t.Project == All || r.Project == All || t.Project == r.Project || t.Project == EmptySelector) &&
		(t.Namespace == All || r.Namespace == All || t.Namespace == r.Namespace) &&
		(t.GuestCluster == All || r.Cluster == All || t.GuestCluster == r.Cluster || t.GuestCluster == EmptySelector) {
		return true
	}

	return false
}
