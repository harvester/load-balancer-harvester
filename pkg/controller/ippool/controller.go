package ippool

import (
	"context"
	"fmt"
	"net"
	"reflect"

	ctlcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/config"
	"github.com/harvester/harvester-load-balancer/pkg/controller/ippool/kubevip"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const controllerName = "harvester-ipam-controller"

type Handler struct {
	ipPoolCache  ctllbv1.IPPoolCache
	ipPoolClient ctllbv1.IPPoolClient
	cmClient     ctlcorev1.ConfigMapClient
	lbCache      ctllbv1.LoadBalancerCache

	allocatorMap           *ipam.SafeAllocatorMap
	kubevipIPPoolConverter *kubevip.IPPoolConverter
}

func Register(ctx context.Context, management *config.Management) error {
	ipPools := management.LbFactory.Loadbalancer().V1beta1().IPPool()
	configmaps := management.CoreFactory.Core().V1().ConfigMap()
	lbCache := management.LbFactory.Loadbalancer().V1beta1().LoadBalancer().Cache()

	handler := &Handler{
		ipPoolCache:            ipPools.Cache(),
		ipPoolClient:           ipPools,
		lbCache:                lbCache,
		allocatorMap:           management.AllocatorMap,
		kubevipIPPoolConverter: kubevip.NewIPPoolConverter(configmaps),
	}

	if err := handler.initializeFromKubevipConfigMap(); err != nil {
		return fmt.Errorf("initialize from kubevip configmap failed, %w", err)
	}

	ipPools.OnChange(ctx, controllerName, handler.OnChange)
	ipPools.OnChange(ctx, controllerName, handler.OnChangeToReleaseAnIP)
	ipPools.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

// OnChange is called when a IPPool is created or updated
// Create a new ipam allocator if the IPPool is new or the ranges are changed
func (h *Handler) OnChange(_ string, ipPool *lbv1.IPPool) (*lbv1.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil {
		return nil, nil
	}

	logrus.Debugf("IP Pool %s has been changed", ipPool.Name)

	previousAllocator := h.allocatorMap.Get(ipPool.Name)
	if previousAllocator == nil || previousAllocator.CheckSum() != ipam.CalculateCheckSum(ipPool.Spec.Ranges) {
		a, err := ipam.NewAllocator(ipPool.Name, ipPool.Spec.Ranges, h.ipPoolCache, h.ipPoolClient)
		if err != nil {
			return nil, err
		}

		// update status
		h.allocatorMap.AddOrUpdate(ipPool.Name, a)
		if err := h.updateStatus(ipPool, a.Total()); err != nil {
			return nil, err
		}
	}

	return ipPool, nil
}

// OnRemove is called when a IPPool is deleted
// Delete the ipam allocator
func (h *Handler) OnRemove(_ string, ipPool *lbv1.IPPool) (*lbv1.IPPool, error) {
	if ipPool == nil {
		return nil, nil
	}
	logrus.Infof("IP Pool %s is deleted", ipPool.Name)
	h.allocatorMap.Delete(ipPool.Name)
	return ipPool, nil
}

// We configure IP pool in the configmap named kubevip in kube-system namespace in v1alpha1 version.
// To be compatible with the v1alpha1 version, we have to initialize IP pool from the kubevip configmap.
func (h *Handler) initializeFromKubevipConfigMap() error {
	logrus.Info("Initialize IP pool from kube-vip configmap")
	pools, err := h.kubevipIPPoolConverter.ConvertFromKubevipConfigMap()
	if err != nil {
		return fmt.Errorf("convert IP pool from kube-vip configmap failed, %w", err)
	}

	for _, pool := range pools {
		if _, err := h.ipPoolClient.Create(pool); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create IP pool %s failed, %w", pool.Name, err)
		}
	}

	return h.kubevipIPPoolConverter.AfterConversion()
}

func (h *Handler) updateStatus(pool *lbv1.IPPool, total int64) error {
	poolCopy := pool.DeepCopy()
	if pool.Status.Allocated == nil {
		poolCopy.Status.Available = total
	} else {
		poolCopy.Status.Available = total - int64(len(pool.Status.Allocated))
	}
	poolCopy.Status.Total = total

	var err error
	poolCopy.Status.AllocatedHistory, err = correctAllocatedHistory(pool)
	if err != nil {
		return fmt.Errorf("correct allocated history for %s failed, %w", pool.Name, err)
	}

	lbv1.IPPoolReady.True(poolCopy)
	lbv1.IPPoolReady.Message(poolCopy, "")

	if reflect.DeepEqual(pool.Status, poolCopy.Status) {
		return nil
	}
	if _, err := h.ipPoolClient.Update(poolCopy); err != nil {
		return fmt.Errorf("update IP pool %s status failed, %w", pool.Name, err)
	}

	return nil
}

func correctAllocatedHistory(pool *lbv1.IPPool) (map[string]string, error) {
	rs, err := ipam.LBRangesToAllocatorRangeSet(pool.Spec.Ranges)
	if err != nil {
		return nil, err
	}

	allocatedHistory := map[string]string{}
	for ipStr := range pool.Status.AllocatedHistory {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return nil, fmt.Errorf("invalid ip %s", ipStr)
		}

		if rs.Contains(ip) {
			allocatedHistory[ipStr] = pool.Status.AllocatedHistory[ipStr]
		}
	}

	return allocatedHistory, nil
}

// Due to previous version bug, the LB may be removed but the related IP still exists on pool allocation record
// This feature gives a way to remove the IP allocation record manually
func (h *Handler) OnChangeToReleaseAnIP(_ string, ipPool *lbv1.IPPool) (*lbv1.IPPool, error) {
	if ipPool == nil || ipPool.DeletionTimestamp != nil || ipPool.Annotations == nil {
		return ipPool, nil
	}

	ipStr := ipPool.Annotations[utils.AnnotationKeyManuallyReleaseIP]
	if ipStr == "" {
		return ipPool, nil
	}

	// check if it is a valid format, e.g.
	// loadbalancer.harvesterhci.io/manuallyReleaseIP: "192.168.5.12: default/cluster1-lb-3"
	ip, namespace, name, err := utils.SplitIPAllocatedString(ipStr)
	if err != nil {
		logrus.Infof("IP Pool %s has a manual IP release request %s, it is not valid, skip", ipPool.Name, ipStr)
		return h.removeAnnotationKeyManuallyReleaseIP(ipPool)
	}

	// check it is a real allocation record
	lbStr := ipPool.Status.Allocated[ip]
	if lbStr == "" || lbStr != fmt.Sprintf("%s/%s", namespace, name) {
		logrus.Infof("IP Pool %s has a manual IP release request %s, it has been released, skip", ipPool.Name, ipStr)
		return h.removeAnnotationKeyManuallyReleaseIP(ipPool)
	}

	// check if the lb is existing
	_, err = h.lbCache.Get(namespace, name)
	if err == nil {
		logrus.Infof("IP Pool %s has a manual IP release request %s, the lb %s/%s is still existing, skip", ipPool.Name, ipStr, namespace, name)
		return h.removeAnnotationKeyManuallyReleaseIP(ipPool)
	}

	// return for retry
	if !apierrors.IsNotFound(err) {
		return ipPool, err
	}

	a := h.allocatorMap.Get(ipPool.Name)
	if a == nil {
		return ipPool, fmt.Errorf("IP Pool %s has a manual IP release request %s, fail to get allocator", ipPool.Name, ipStr)
	}
	err = a.Release(fmt.Sprintf("%s/%s", namespace, name), "")
	if err != nil {
		return ipPool, fmt.Errorf("IP Pool %s has a manual IP release request %s, fail to release, error:%w", ipPool.Name, ipStr, err)
	}

	logrus.Infof("IP Pool %s has a manual IP release request %s, it is successfully released", ipPool.Name, ipStr)

	// note: above a.Release also updates pool internally, following call may fail at first time, as ipPool is stale
	return h.removeAnnotationKeyManuallyReleaseIP(ipPool)
}

func (h *Handler) removeAnnotationKeyManuallyReleaseIP(ipPool *lbv1.IPPool) (*lbv1.IPPool, error) {
	ipPoolCopy := ipPool.DeepCopy()
	delete(ipPoolCopy.Annotations, utils.AnnotationKeyManuallyReleaseIP)
	return h.ipPoolClient.Update(ipPoolCopy)
}
