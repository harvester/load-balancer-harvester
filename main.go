//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go
//go:generate /bin/bash scripts/generate-manifest

package main

import (
	"context"
	"flag"

	ctlCore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/harvester/harvester-load-balancer/pkg/controller/loadbalancer"
	"github.com/harvester/harvester-load-balancer/pkg/controller/service"
	ctlDiscovery "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io"
	ctlLB "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io"
)

var (
	masterURL      string
	kubeconfigFile string
)

func init() {
	flag.StringVar(&kubeconfigFile, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.Parse()
}

func main() {
	// set up signals so we handle the first shutdown signal gracefully
	ctx := signals.SetupSignalHandler(context.Background())

	// This will load the kubeconfig file in a style the same as kubectl
	cfg, err := kubeconfig.GetNonInteractiveClientConfig(kubeconfigFile).ClientConfig()
	if err != nil {
		logrus.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	// Generated lb controller
	lbFactory := ctlLB.NewFactoryFromConfigOrDie(cfg)
	coreFactory := ctlCore.NewFactoryFromConfigOrDie(cfg)
	discoveryFactory := ctlDiscovery.NewFactoryFromConfigOrDie(cfg)
	client := kubernetes.NewForConfigOrDie(cfg)

	leader.RunOrDie(ctx, "kube-system", "harvester-load-balancer", client, func(ctx context.Context) {
		if err := loadbalancer.Register(ctx, lbFactory, coreFactory, discoveryFactory); err != nil {
			logrus.Fatalf("Error register loadBalancer controller: %s", err.Error())
		}
		if err := service.Register(ctx, lbFactory, coreFactory); err != nil {
			logrus.Fatalf("Error register service controller: %s", err.Error())
		}
		// Start all the controllers
		if err := start.All(ctx, 2, lbFactory, coreFactory); err != nil {
			logrus.Fatalf("Error starting: %s", err.Error())
		}
	})

	<-ctx.Done()
}
