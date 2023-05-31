package loadbalancer

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"time"

	ctlcniv1 "github.com/harvester/harvester/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	ctlCorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"

	lb "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io"
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

	AnnotationKeyNetwork   = lb.GroupName + "/network"
	AnnotationKeyProject   = lb.GroupName + "/project"
	AnnotationKeyNamespace = lb.GroupName + "/namespace"
	AnnotationKeyCluster   = lb.GroupName + "/cluster"

	defaultWaitIPTimeout = time.Second * 5
)

type Handler struct {
	lbClient            ctllbv1.LoadBalancerClient
	ipPoolCache         ctllbv1.IPPoolCache
	nadCache            ctlcniv1.NetworkAttachmentDefinitionCache
	serviceClient       ctlCorev1.ServiceClient
	serviceCache        ctlCorev1.ServiceCache
	endpointSliceClient ctldiscoveryv1.EndpointSliceClient
	endpointSliceCache  ctldiscoveryv1.EndpointSliceCache
	vmiCache            ctlkubevirtv1.VirtualMachineInstanceCache

	allocatorMap *ipam.SafeAllocatorMap

	lbManager lbpkg.Manager
}

func Register(ctx context.Context, management *config.Management) error {
	lbs := management.LbFactory.Loadbalancer().V1beta1().LoadBalancer()
	pools := management.LbFactory.Loadbalancer().V1beta1().IPPool()
	nads := management.CniFactory.K8s().V1().NetworkAttachmentDefinition()
	services := management.CoreFactory.Core().V1().Service()
	endpointSlices := management.DiscoveryFactory.Discovery().V1().EndpointSlice()
	vmis := management.KubevirtFactory.Kubevirt().V1().VirtualMachineInstance()

	handler := &Handler{
		lbClient:            lbs,
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

	lbs.OnChange(ctx, controllerName, handler.OnChange)
	lbs.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(_ string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil || lb.DeletionTimestamp != nil || lb.APIVersion != lbv1.SchemeGroupVersion.String() {
		return nil, nil
	}
	logrus.Debugf("load balancer %s/%s has been changed, spec: %+v, apiVersion: %s", lb.Namespace, lb.Name, lb.Spec, lb.APIVersion)

	lbCopy := lb.DeepCopy()
	allocatedAddress, err := h.allocateIP(lb)
	if err != nil {
		err = fmt.Errorf("allocate ip for lb %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
		return h.updateStatus(lbCopy, lb, err)
	}
	if allocatedAddress != nil {
		lbCopy.Status.AllocatedAddress = *allocatedAddress
	}
	// The workload type defaults to VM if not specified to be compatible with previous versions
	if lb.Spec.WorkloadType == lbv1.VM || lb.Spec.WorkloadType == "" {
		if err = h.ensureVMLoadBalancer(lbCopy); err != nil {
			return h.updateStatus(lbCopy, lb, err)
		}
	}

	return h.updateStatus(lbCopy, lb, nil)
}

func (h *Handler) ensureVMLoadBalancer(lb *lbv1.LoadBalancer) error {
	if err := h.lbManager.EnsureLoadBalancer(lb); err != nil {
		return fmt.Errorf("ensure load balancer %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
	}
	ip, err := h.waitServiceExternalIP(lb.Namespace, lb.Name)
	if err != nil {
		return fmt.Errorf("wait service %s/%s external ip failed, error: %w", lb.Namespace, lb.Name, err)
	}
	lb.Status.Address = ip
	servers, err := h.getBackendServers(lb)
	if err != nil {
		return fmt.Errorf("get backend servers of lb %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
	}
	lb.Status.BackendServers = servers

	return nil
}

func (h *Handler) updateStatus(lbCopy, lb *lbv1.LoadBalancer, err error) (*lbv1.LoadBalancer, error) {
	if err != nil {
		lbv1.LoadBalancerReady.False(lbCopy)
		lbv1.LoadBalancerReady.Message(lbCopy, err.Error())
	} else {
		lbv1.LoadBalancerReady.True(lbCopy)
		lbv1.LoadBalancerReady.Message(lbCopy, "")
	}

	// status didn't change, don't update it
	if reflect.DeepEqual(lbCopy.Status, lb.Status) {
		return lbCopy, err
	}

	updatedLb, updatedErr := h.lbClient.Update(lbCopy)
	if updatedErr != nil {
		return nil, fmt.Errorf("update lb %s/%s status failed, error: %w", lb.Namespace, lb.Name, updatedErr)
	}

	return updatedLb, err
}

func (h *Handler) getBackendServers(lb *lbv1.LoadBalancer) ([]string, error) {
	backendServers, err := h.lbManager.GetBackendServers(lb)
	if err != nil {
		return nil, err
	}

	servers := make([]string, 0, len(backendServers))
	for _, server := range backendServers {
		addr, ok := server.GetAddress()
		if ok {
			servers = append(servers, addr)
		}
	}

	return servers, nil
}

func (h *Handler) OnRemove(_ string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil {
		return nil, nil
	}

	logrus.Debugf("load balancer %s/%s has been deleted", lb.Namespace, lb.Name)

	if lb.Spec.IPAM == lbv1.Pool && lb.Status.AllocatedAddress.IPPool != "" {
		if err := h.releaseIP(lb); err != nil {
			return nil, fmt.Errorf("release ip of lb %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
		}
	}

	if lb.Spec.WorkloadType == lbv1.VM {
		if err := h.lbManager.DeleteLoadBalancer(lb); err != nil {
			return nil, fmt.Errorf("delete lb %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
		}
	}

	return lb, nil
}

func (h *Handler) allocateIP(lb *lbv1.LoadBalancer) (*lbv1.AllocatedAddress, error) {
	allocated := lb.Status.AllocatedAddress

	if lb.Spec.IPAM == lbv1.DHCP {
		return h.allocatedIPFromDHCP(&allocated, lb)
	}

	return h.allocatedIPFromPool(&allocated, lb)
}

func (h *Handler) allocatedIPFromDHCP(allocated *lbv1.AllocatedAddress, lb *lbv1.LoadBalancer) (*lbv1.AllocatedAddress, error) {
	var err error
	// release the IP if the lb has applied an IP
	if allocated.IPPool != "" {
		if err = h.releaseIP(lb); err != nil {
			return nil, err
		}
	}

	if allocated.IP != utils.Address4AskDHCP {
		return &lbv1.AllocatedAddress{
			IP: utils.Address4AskDHCP,
		}, nil
	}

	return nil, nil
}

func (h *Handler) allocatedIPFromPool(allocated *lbv1.AllocatedAddress, lb *lbv1.LoadBalancer) (*lbv1.AllocatedAddress, error) {
	var err error
	pool := lb.Spec.IPPool
	if pool == "" {
		// match an IP pool automatically if not specified
		pool, err = h.selectIPPool(lb)
		if err != nil {
			return nil, fmt.Errorf("fail to select the pool for lb %s/%s, error: %w", lb.Namespace, lb.Name, err)
		}
	}
	// release the IP from other IP pool
	if allocated.IPPool != "" && allocated.IPPool != pool {
		if err = h.releaseIP(lb); err != nil {
			return nil, err
		}
	}
	if allocated.IPPool != pool {
		return h.requestIP(lb, pool)
	}

	return nil, nil
}

func (h *Handler) requestIP(lb *lbv1.LoadBalancer, pool string) (*lbv1.AllocatedAddress, error) {
	// get allocator
	allocator := h.allocatorMap.Get(pool)
	if allocator == nil {
		return nil, fmt.Errorf("could not get the allocator %s", pool)
	}
	// get IP
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
		Network:   lb.Annotations[AnnotationKeyNetwork],
		Project:   lb.Annotations[AnnotationKeyProject],
		Namespace: lb.Annotations[AnnotationKeyNamespace],
		Cluster:   lb.Annotations[AnnotationKeyCluster],
	}
	if r.Namespace == "" {
		r.Namespace = lb.Namespace
	}
	pool, err := ipam.NewSelector(h.ipPoolCache).Select(r)
	if err != nil {
		return "", fmt.Errorf("select IP pool failed, error: %w", err)
	}
	if pool == nil {
		return "", fmt.Errorf("no matching IP pool with requirement %+v", r)
	}

	return pool.Name, nil
}

func (h *Handler) releaseIP(lb *lbv1.LoadBalancer) error {
	a := h.allocatorMap.Get(lb.Status.AllocatedAddress.IPPool)
	if a == nil {
		return fmt.Errorf("could not get the allocator %s", lb.Status.AllocatedAddress.IPPool)
	}
	return a.Release(fmt.Sprintf("%s/%s", lb.Namespace, lb.Name), "")
}

func (h *Handler) waitServiceExternalIP(namespace, name string) (string, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	tick := ticker.C
	timeout := time.After(defaultWaitIPTimeout)

	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout")
		case <-tick:
			svc, err := h.serviceCache.Get(namespace, name)
			if err != nil {
				logrus.Warnf("get service %s/%s failed, error: %v, continue...", namespace, name, err)
				continue
			}
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				return svc.Status.LoadBalancer.Ingress[0].IP, nil
			}
		}
	}
}
