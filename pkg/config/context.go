package config

import (
	"context"

	ctlcni "github.com/harvester/harvester/pkg/generated/controllers/k8s.cni.cncf.io"
	ctlkubevirt "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io"
	ctlapiext "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io"
	ctlcore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/start"
	"k8s.io/client-go/rest"

	ctldiscovery "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io"
	ctllb "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io"
	"github.com/harvester/harvester-load-balancer/pkg/ipam"
	"github.com/harvester/harvester-load-balancer/pkg/lb"
	"github.com/harvester/harvester-load-balancer/pkg/lb/servicelb"
)

type Management struct {
	Ctx context.Context

	CoreFactory      *ctlcore.Factory
	LbFactory        *ctllb.Factory
	CniFactory       *ctlcni.Factory
	DiscoveryFactory *ctldiscovery.Factory
	KubevirtFactory  *ctlkubevirt.Factory
	ApiextFactory    *ctlapiext.Factory

	starters []start.Starter

	AllocatorMap *ipam.SafeAllocatorMap

	LBManager lb.Manager
}

func SetupManagement(ctx context.Context, cfg *rest.Config) *Management {
	lbFactory := ctllb.NewFactoryFromConfigOrDie(cfg)
	cniFactory := ctlcni.NewFactoryFromConfigOrDie(cfg)
	coreFactory := ctlcore.NewFactoryFromConfigOrDie(cfg)
	discoveryFactory := ctldiscovery.NewFactoryFromConfigOrDie(cfg)
	kubevirtFactory := ctlkubevirt.NewFactoryFromConfigOrDie(cfg)

	management := &Management{
		Ctx: ctx,

		CoreFactory:      coreFactory,
		LbFactory:        lbFactory,
		CniFactory:       cniFactory,
		DiscoveryFactory: discoveryFactory,
		KubevirtFactory:  kubevirtFactory,

		AllocatorMap: ipam.NewSafeAllocatorMap(),
	}

	serviceController := coreFactory.Core().V1().Service()
	epsController := discoveryFactory.Discovery().V1().EndpointSlice()
	vmiController := kubevirtFactory.Kubevirt().V1().VirtualMachineInstance()

	management.LBManager = servicelb.NewManager(ctx, serviceController, serviceController.Cache(),
		epsController, epsController.Cache(), vmiController.Cache())

	management.starters = append(management.starters, coreFactory, discoveryFactory, cniFactory, lbFactory, kubevirtFactory)

	return management
}

func (m *Management) Start(threadiness int) error {
	for _, starter := range m.starters {
		if err := starter.Start(m.Ctx, threadiness); err != nil {
			return err
		}
	}

	return nil
}
