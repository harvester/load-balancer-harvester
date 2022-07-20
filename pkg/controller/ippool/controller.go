package ippool

import (
	"context"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	"github.com/harvester/harvester-load-balancer/pkg/config"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1alpha1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam"
)

const controllerName = "harvester-ipam-controller"

type Handler struct {
	ipPoolCache  ctllbv1.IPPoolCache
	ipPoolClient ctllbv1.IPPoolClient

	allocatorMap *ipam.SafeAllocatorMap
}

func Register(ctx context.Context, management *config.Management) error {
	ipPools := management.LbFactory.Loadbalancer().V1alpha1().IPPool()

	handler := &Handler{
		ipPoolCache:  ipPools.Cache(),
		ipPoolClient: ipPools,

		allocatorMap: management.AllocatorMap,
	}

	if err := handler.initialize(); err != nil {
		return err
	}

	ipPools.OnChange(ctx, controllerName, handler.OnChange)
	ipPools.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(_ string, ipPool *lbv1.IPPool) (*lbv1.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil {
		return nil, nil
	}

	logrus.Infof("IP Pool %s has been changed", ipPool.Name)

	previousAllocator := h.allocatorMap.Get(ipPool.Name)
	if previousAllocator == nil || previousAllocator.CheckSum() != ipam.CalculateCheckSum(ipPool.Spec.Ranges) {
		a, err := ipam.NewAllocator(ipPool.Name, ipPool.Spec.Ranges, h.ipPoolCache, h.ipPoolClient)
		if err != nil {
			return nil, err
		}

		// update status
		h.allocatorMap.AddOrUpdate(ipPool.Name, a)
		if err := h.initialStatus(ipPool, a.Total()); err != nil {
			return nil, err
		}
	}

	return ipPool, nil
}

func (h *Handler) OnRemove(_ string, ipPool *lbv1.IPPool) (*lbv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}

	logrus.Infof("IP Pool %s has been deleted", ipPool.Name)

	h.allocatorMap.Delete(ipPool.Name)

	return ipPool, nil
}

func (h *Handler) initialize() error {
	pools, err := h.ipPoolClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pool := range pools.Items {
		a, err := ipam.NewAllocator(pool.Name, pool.Spec.Ranges, h.ipPoolCache, h.ipPoolClient)
		if err != nil {
			return err
		}
		h.allocatorMap.AddOrUpdate(pool.Name, a)
		if err := h.initialStatus(&pool, a.Total()); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) initialStatus(ipPool *lbv1.IPPool, total int64) error {
	ipPoolCopy := ipPool.DeepCopy()
	if ipPool.Status.Allocated == nil {
		ipPoolCopy.Status.Available = total
	} else {
		ipPoolCopy.Status.Available = total - int64(len(ipPool.Status.Allocated))
	}
	ipPoolCopy.Status.Total = total
	lbv1.IPPoolReady.True(ipPoolCopy)
	lbv1.IPPoolReady.Message(ipPoolCopy, "")

	if _, err := h.ipPoolClient.Update(ipPoolCopy); err != nil {
		return err
	}

	return nil
}
