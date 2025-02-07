package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	ctlcniv1 "github.com/harvester/harvester/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/config"
	ctldiscoveryv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io/v1"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam"
	lbpkg "github.com/harvester/harvester-load-balancer/pkg/lb"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const (
	controllerName = "harvester-lb-controller"

	// referred by cloud-provider-harvester
	AnnotationKeyNetwork   = utils.AnnotationKeyNetwork
	AnnotationKeyProject   = utils.AnnotationKeyProject
	AnnotationKeyNamespace = utils.AnnotationKeyNamespace
	AnnotationKeyCluster   = utils.AnnotationKeyCluster
)

var (
	errNoMatchedIPPool             = errors.New("no matched IPPool")
	errNoAvailableIP               = errors.New("no available IP")
	errNoRunningBackendServer      = errors.New("no running backend servers")
	errAllBackendServersNotHealthy = errors.New("running backend servers are not probed as healthy")
)

type Handler struct {
	lbController        ctllbv1.LoadBalancerController
	ipPoolCache         ctllbv1.IPPoolCache
	nadCache            ctlcniv1.NetworkAttachmentDefinitionCache
	serviceClient       ctlcorev1.ServiceClient
	serviceCache        ctlcorev1.ServiceCache
	endpointSliceClient ctldiscoveryv1.EndpointSliceClient
	endpointSliceCache  ctldiscoveryv1.EndpointSliceCache
	vmiCache            ctlkubevirtv1.VirtualMachineInstanceCache

	allocatorMap *ipam.SafeAllocatorMap

	lbManager lbpkg.Manager
}

func Register(ctx context.Context, management *config.Management) error {
	lbc := management.LbFactory.Loadbalancer().V1beta1().LoadBalancer()
	pools := management.LbFactory.Loadbalancer().V1beta1().IPPool()
	nads := management.CniFactory.K8s().V1().NetworkAttachmentDefinition()
	services := management.CoreFactory.Core().V1().Service()
	endpointSlices := management.DiscoveryFactory.Discovery().V1().EndpointSlice()
	vmis := management.KubevirtFactory.Kubevirt().V1().VirtualMachineInstance()

	handler := &Handler{
		lbController:        lbc,
		ipPoolCache:         pools.Cache(),
		nadCache:            nads.Cache(),
		serviceClient:       services,
		serviceCache:        services.Cache(),
		endpointSliceClient: endpointSlices,
		endpointSliceCache:  endpointSlices.Cache(),
		vmiCache:            vmis.Cache(),

		allocatorMap: management.AllocatorMap,

		lbManager: management.LBManager,
	}

	// NOTE: register the health check hander BEFORE the controller starts working
	// no mutex is used to protect
	if err := handler.lbManager.RegisterHealthCheckHandler(handler.HealthCheckNotify); err != nil {
		return err
	}

	lbc.OnChange(ctx, controllerName, handler.OnChange)
	lbc.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(_ string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil || lb.DeletionTimestamp != nil || lb.APIVersion != lbv1.SchemeGroupVersion.String() {
		return nil, nil
	}
	logrus.Debugf("lb %s/%s is changed, spec: %+v, apiVersion: %s", lb.Namespace, lb.Name, lb.Spec, lb.APIVersion)

	lbCopy := lb.DeepCopy()

	// 1. ensure lb get an address
	if lb, err := h.ensureAllocatedAddress(lbCopy, lb); err != nil {
		return h.handleError(lbCopy, lb, err)
	}

	// 2. ensure lb's implementation when it is VM type
	// The workload type defaults to VM if not specified to be compatible with previous versions
	if lb.Spec.WorkloadType == lbv1.VM || lb.Spec.WorkloadType == "" {
		if lb, err := h.ensureVMLoadBalancer(lbCopy, lb); err != nil {
			return h.handleError(lbCopy, lb, err)
		}
	}

	// move lb to Ready
	return h.updateStatus(lbCopy, lb, nil)
}

func (h *Handler) OnRemove(_ string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil {
		return nil, nil
	}
	logrus.Infof("lb %s/%s is deleted, address %s, allocatedIP %s", lb.Namespace, lb.Name, lb.Status.Address, lb.Status.AllocatedAddress.IP)

	if lb.Spec.IPAM == lbv1.Pool && lb.Status.AllocatedAddress.IPPool != "" {
		if err := h.releaseIP(lb); err != nil {
			logrus.Infof("lb %s/%s fail to release ip %s, error: %s", lb.Namespace, lb.Name, lb.Status.AllocatedAddress.IP, err.Error())
			return nil, fmt.Errorf("fail to release ip %s, error: %w", lb.Status.AllocatedAddress.IP, err)
		}
		logrus.Debugf("lb %s/%s release ip %s", lb.Namespace, lb.Name, lb.Status.AllocatedAddress.IP)
	}

	if lb.Spec.WorkloadType == lbv1.VM || lb.Spec.WorkloadType == "" {
		if err := h.lbManager.DeleteLoadBalancer(lb); err != nil {
			logrus.Infof("lb %s/%s fail to delete service, error: %s", lb.Namespace, lb.Name, err.Error())
			return nil, fmt.Errorf("fail to delete service, error: %w", err)
		}
		logrus.Debugf("lb %s/%s delete service", lb.Namespace, lb.Name)
	}

	return lb, nil
}

func (h *Handler) handleError(lbCopy, lb *lbv1.LoadBalancer, err error) (*lbv1.LoadBalancer, error) {
	// handle customized error
	if errors.Is(err, errNoMatchedIPPool) || errors.Is(err, errNoAvailableIP) || errors.Is(err, lbpkg.ErrWaitExternalIP) {
		h.lbController.EnqueueAfter(lb.Namespace, lb.Name, 1*time.Second)
		return h.updateStatusNotReturnError(lbCopy, lb, err)
	} else if errors.Is(err, errNoRunningBackendServer) || errors.Is(err, errAllBackendServersNotHealthy) {
		// stop reconciler, wait vmi controller Enqueue() lb / health check go thread Enqueue()
		return h.updateStatusNotReturnError(lbCopy, lb, err)
	}
	return h.updateStatus(lbCopy, lb, err)
}

func (h *Handler) ensureAllocatedAddress(lbCopy, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb.Spec.IPAM == lbv1.DHCP {
		return h.ensureAllocatedAddressDHCP(lbCopy, lb)
	}
	return h.ensureAllocatedAddressPool(lbCopy, lb)
}

func (h *Handler) ensureVMLoadBalancer(lbCopy, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if err := h.lbManager.EnsureLoadBalancer(lb); err != nil {
		return lb, err
	}

	ip, err := h.lbManager.EnsureLoadBalancerServiceIP(lb)
	if err != nil {
		lbCopy.Status.Address = ""
		return lb, err
	}

	lbCopy.Status.Address = ip
	servers, err := h.lbManager.EnsureBackendServers(lb)
	if err != nil {
		return lb, err
	}
	lbCopy.Status.BackendServers = getServerAddress(servers)
	if len(lbCopy.Status.BackendServers) == 0 {
		return lb, errNoRunningBackendServer
	}

	if lb.Spec.HealthCheck != nil && lb.Spec.HealthCheck.Port != 0 {
		count, err := h.lbManager.GetProbeReadyBackendServerCount(lb)
		if err != nil {
			return lb, err
		}

		logrus.Debugf("lb %s/%s active probe count %v", lb.Namespace, lb.Name, count)
		if count == 0 {
			return lb, fmt.Errorf("%w total:%v, healthy:0", errAllBackendServersNotHealthy, len(lbCopy.Status.BackendServers))
		}
	}

	return lb, nil
}

func getServerAddress(servers []lbpkg.BackendServer) []string {
	if len(servers) == 0 {
		return nil
	}
	address := make([]string, 0, len(servers))
	for _, server := range servers {
		if addr, ok := server.GetAddress(); ok {
			address = append(address, addr)
		}
	}
	return address
}

func (h *Handler) updateStatus(lbCopy, lb *lbv1.LoadBalancer, err error) (*lbv1.LoadBalancer, error) {
	if err != nil {
		lbv1.LoadBalancerReady.False(lbCopy)
		lbv1.LoadBalancerReady.Message(lbCopy, err.Error())
	} else {
		lbv1.LoadBalancerReady.True(lbCopy)
		lbv1.LoadBalancerReady.Message(lbCopy, "")
	}

	// don't update when no change happens
	if reflect.DeepEqual(lbCopy.Status, lb.Status) {
		return lbCopy, err
	}
	updatedLb, updatedErr := h.lbController.Update(lbCopy)
	if updatedErr != nil {
		return nil, fmt.Errorf("fail to update status, error: %w", updatedErr)
	}

	return updatedLb, err
}

// do not return error to wrangler framework, avoid endless error message
// caller decides where to add Enqueue
func (h *Handler) updateStatusNotReturnError(lbCopy, lb *lbv1.LoadBalancer, err error) (*lbv1.LoadBalancer, error) {
	// set status to False
	lbv1.LoadBalancerReady.False(lbCopy)
	lbv1.LoadBalancerReady.Message(lbCopy, err.Error())

	// don't update when no change happens
	if reflect.DeepEqual(lbCopy.Status, lb.Status) {
		return lb, nil
	}

	updatedLb, updatedErr := h.lbController.Update(lbCopy)
	if updatedErr != nil {
		return nil, fmt.Errorf("fail to update status with original error %w, new error: %w", err, updatedErr)
	}
	logrus.Infof("lb %s/%s is set to not ready, error: %s", lb.Namespace, lb.Name, err.Error())
	return updatedLb, nil
}

func (h *Handler) ensureAllocatedAddressDHCP(lbCopy, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb.Status.AllocatedAddress.IP != utils.Address4AskDHCP {
		lbCopy.Status.AllocatedAddress = lbv1.AllocatedAddress{
			IP: utils.Address4AskDHCP,
		}
		return lb, nil
	}

	return lb, nil
}

func (h *Handler) ensureAllocatedAddressPool(lbCopy, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	// lb's ip pool changes, release the previous allocated IP
	if lb.Spec.IPPool != "" && lb.Status.AllocatedAddress.IPPool != "" && lb.Status.AllocatedAddress.IPPool != lb.Spec.IPPool {
		logrus.Infof("lb %s/%s release ip %s to pool %s", lb.Namespace, lb.Name, lb.Status.AllocatedAddress.IP, lb.Status.AllocatedAddress.IPPool)
		if err := h.releaseIP(lb); err != nil {
			logrus.Warnf("lb %s/%s fail to release ip %s to pool %s, error: %s", lb.Namespace, lb.Name, lb.Status.AllocatedAddress.IP, lb.Spec.IPPool, err.Error())
			return lb, fmt.Errorf("fail to release ip %s to pool %s, error: %w", lb.Status.AllocatedAddress.IP, lb.Spec.IPPool, err)
		}

		lbCopy.Status.AllocatedAddress = lbv1.AllocatedAddress{}
		return lb, nil
	}

	// allocate or re-allocate IP
	if lb.Status.AllocatedAddress.IPPool == "" {
		ip, err := h.allocateIPFromPool(lb)
		if err != nil {
			logrus.Debugf("lb %s/%s fail to allocate from pool %s", lb.Namespace, lb.Name, err.Error())
			// if unlucky the DuplicateAllocationKeyWord is reported, try to release IP, do not overwrite original error
			if strings.Contains(err.Error(), utils.DuplicateAllocationKeyWord) {
				pool, releaseErr := h.tryReleaseDuplicatedIPToPool(lb)
				if releaseErr != nil {
					logrus.Infof("lb %s/%s error: %s, try to release ip to pool %s, error: %s", lb.Namespace, lb.Name, err.Error(), pool, releaseErr.Error())
				} else {
					logrus.Infof("lb %s/%s error: %s, try to release ip to pool %s, ok", lb.Namespace, lb.Name, err.Error(), pool)
				}
			}
			return lb, err
		}

		lbCopy.Status.AllocatedAddress = *ip
		logrus.Infof("lb %s/%s allocate ip %s from pool %s", lb.Namespace, lb.Name, ip.IP, ip.IPPool)
		return lb, nil
	}

	return lb, nil
}

func (h *Handler) allocateIPFromPool(lb *lbv1.LoadBalancer) (*lbv1.AllocatedAddress, error) {
	pool := lb.Spec.IPPool
	if pool == "" {
		// match an IP pool automatically if not specified
		pool, err := h.selectIPPool(lb)
		if err != nil {
			return nil, err
		}
		return h.requestIP(lb, pool)
	}

	return h.requestIP(lb, pool)
}

func (h *Handler) tryReleaseDuplicatedIPToPool(lb *lbv1.LoadBalancer) (string, error) {
	pool := lb.Spec.IPPool
	if pool == "" {
		// match an IP pool automatically if not specified
		pool, err := h.selectIPPool(lb)
		if err != nil {
			return pool, err
		}
	}

	// if pool is not ready, just fail and wait
	a := h.allocatorMap.Get(pool)
	if a == nil {
		return pool, fmt.Errorf("fail to get allocator %s", pool)
	}
	return pool, a.Release(fmt.Sprintf("%s/%s", lb.Namespace, lb.Name), "")
}

func (h *Handler) requestIP(lb *lbv1.LoadBalancer, pool string) (*lbv1.AllocatedAddress, error) {
	allocator := h.allocatorMap.Get(pool)
	if allocator == nil {
		return nil, fmt.Errorf("fail to get allocator %s", pool)
	}
	// the ip is booked on pool when successfully Get()
	ipConfig, err := allocator.Get(fmt.Sprintf("%s/%s", lb.Namespace, lb.Name))
	if err != nil {
		return nil, err
	}

	return &lbv1.AllocatedAddress{
		IPPool:  pool,
		IP:      ipConfig.Address.IP.String(),
		Mask:    net.IP(ipConfig.Address.Mask).String(),
		Gateway: ipConfig.Gateway.String(),
	}, nil
}

func (h *Handler) selectIPPool(lb *lbv1.LoadBalancer) (string, error) {
	r := &ipam.Requirement{
		Network:   lb.Annotations[utils.AnnotationKeyNetwork],
		Project:   lb.Annotations[utils.AnnotationKeyProject],
		Namespace: lb.Annotations[utils.AnnotationKeyNamespace],
		Cluster:   lb.Annotations[utils.AnnotationKeyCluster],
	}
	if r.Namespace == "" {
		r.Namespace = lb.Namespace
	}
	pool, err := ipam.NewSelector(h.ipPoolCache).Select(r)
	if err != nil {
		return "", fmt.Errorf("%w with selector, error: %w", errNoMatchedIPPool, err)
	}
	if pool == nil {
		return "", fmt.Errorf("%w with requirement %+v", errNoMatchedIPPool, r)
	}

	return pool.Name, nil
}

func (h *Handler) releaseIP(lb *lbv1.LoadBalancer) error {
	// if pool is not ready, just fail and wait
	a := h.allocatorMap.Get(lb.Status.AllocatedAddress.IPPool)
	if a == nil {
		return fmt.Errorf("fail to get allocator %s", lb.Status.AllocatedAddress.IPPool)
	}
	return a.Release(fmt.Sprintf("%s/%s", lb.Namespace, lb.Name), "")
}

// lb manager health check notify that the health of some VMs changed
func (h *Handler) HealthCheckNotify(namespace, name string) error {
	h.lbController.Enqueue(namespace, name)
	return nil
}
