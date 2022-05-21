package loadbalancer

import (
	"context"
	"fmt"
	"net"
	"time"

	lb "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io"
	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	"github.com/harvester/harvester-load-balancer/pkg/config"
	ctldiscoveryv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io/v1"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1alpha1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam"
	lbpkg "github.com/harvester/harvester-load-balancer/pkg/lb"
	ctlcniv1 "github.com/harvester/harvester/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	ctlCorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
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
	lbs := management.LbFactory.Loadbalancer().V1alpha1().LoadBalancer()
	pools := management.LbFactory.Loadbalancer().V1alpha1().IPPool()
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
	if lb == nil || lb.DeletionTimestamp != nil {
		return nil, nil
	}
	logrus.Infof("load balancer %s/%s has been changed, spec: %+v", lb.Namespace, lb.Name, lb.Spec)

	lbCopy := lb.DeepCopy()
	allocatedAddress, err := h.allocateIP(lb)
	if err != nil {
		return nil, err
	}
	if allocatedAddress != nil {
		lbCopy.Status.AllocatedAddress = *allocatedAddress
	}

	if lb.Spec.WorkloadType == lbv1.VM {
		if err := h.lbManager.EnsureLoadBalancer(lbCopy); err != nil {
			return nil, err
		}
		ip, err := h.waitServiceExternalIP(lb.Namespace, lb.Name)
		if err != nil {
			return nil, err
		}
		lbCopy.Status.Address = ip
	}

	if lbCopy != nil {
		lbv1.LoadBalancerReady.True(lbCopy)
		lbv1.LoadBalancerReady.Message(lbCopy, "")
		return h.lbClient.Update(lbCopy)
	}

	return lb, nil
}

func (h *Handler) OnRemove(_ string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil {
		return nil, nil
	}

	logrus.Infof("load balancer %s/%s has been deleted", lb.Namespace, lb.Name)

	if lb.Spec.IPAM == lbv1.Pool && lb.Status.AllocatedAddress.IPPool != "" {
		if err := h.releaseIP(lb); err != nil {
			return nil, err
		}
	}

	if lb.Spec.WorkloadType == lbv1.VM {
		if err := h.lbManager.DeleteLoadBalancer(lb); err != nil {
			return nil, err
		}
	}

	return lb, nil
}

func (h *Handler) allocateIP(lb *lbv1.LoadBalancer) (*lbv1.AllocatedAddress, error) {
	allocated := lb.Status.AllocatedAddress
	var err error

	if lb.Spec.IPAM == lbv1.DHCP {
		// release the IP if the lb has applied an IP
		if allocated.IPPool != "" {
			if err = h.releaseIP(lb); err != nil {
				return nil, err
			}
		}
		if allocated.IP != ipam.Address4AskDHCP {
			return &lbv1.AllocatedAddress{
				IP: ipam.Address4AskDHCP,
			}, nil
		}
		return nil, nil
	}

	// If lb.Spec.IPAM equals pool
	pool := lb.Spec.IPPool
	if pool == "" {
		// match an IP pool automatically if not specified
		pool, err = h.selectIPPool(lb)
		if err != nil {
			return nil, fmt.Errorf("fail to select the pool for lb %s/%s", lb.Namespace, lb.Name)
		}
	}
	// release the IP from other IP pool
	if allocated.IPPool != "" && allocated.IPPool != pool {
		if err := h.releaseIP(lb); err != nil {
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
	}, err
}

func (h *Handler) selectIPPool(lb *lbv1.LoadBalancer) (string, error) {
	r := &ipam.Requirement{
		Network:   lb.Annotations[AnnotationKeyNetwork],
		Project:   lb.Annotations[AnnotationKeyProject],
		Namespace: lb.Annotations[AnnotationKeyNamespace],
		Cluster:   lb.Annotations[AnnotationKeyCluster],
	}
	pool, err := ipam.NewSelector(h.ipPoolCache).Select(r)
	if err != nil {
		return "", err
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

	for {
		select {
		case <-time.After(defaultWaitIPTimeout):
			return "", fmt.Errorf("wait IP timeout")
		case <-tick:
			svc, err := h.serviceCache.Get(namespace, name)
			if err != nil {
				continue
			}
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				return svc.Status.LoadBalancer.Ingress[0].IP, nil
			}
		}
	}
}
