//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go

package main

import (
	"context"
	"flag"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/harvester/harvester-conveyor/pkg/controller/loadbalancer"
	"github.com/harvester/harvester-conveyor/pkg/generated/controllers/core"
	"github.com/harvester/harvester-conveyor/pkg/generated/controllers/network.harvesterhci.io"
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
	lbFactory := network.NewFactoryFromConfigOrDie(cfg)
	coreFactory := core.NewFactoryFromConfigOrDie(cfg)
	client := kubernetes.NewForConfigOrDie(cfg)

	leader.RunOrDie(ctx, "kube-system", "harvester-conveyor", client, func(ctx context.Context) {
		if err := loadbalancer.Register(ctx, lbFactory, coreFactory); err != nil {
			logrus.Fatalf("Error register load balancer: %s", err.Error())
		}
		// Start all the controllers
		if err := start.All(ctx, 2, lbFactory); err != nil {
			logrus.Fatalf("Error starting: %s", err.Error())
		}
	})

	<-ctx.Done()
}
