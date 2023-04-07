//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go
//go:generate /bin/bash scripts/generate-manifest

package main

import (
	"context"
	"os"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/harvester/harvester-load-balancer/pkg/config"
	"github.com/harvester/harvester-load-balancer/pkg/controller/ippool"
	"github.com/harvester/harvester-load-balancer/pkg/controller/loadbalancer"
	"github.com/harvester/harvester-load-balancer/pkg/controller/vmi"
)

func main() {
	// set up signals so we handle the first shutdown signal gracefully
	ctx := signals.SetupSignalContext()

	// This will load the kubeconfig file in a style the same as kubectl
	cfg, err := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG")).ClientConfig()
	if err != nil {
		logrus.Fatalf("error building kubeconfig: %s", err.Error())
	}

	// Generated lb controller
	management := config.SetupManagement(ctx, cfg)
	client := kubernetes.NewForConfigOrDie(cfg)

	leader.RunOrDie(ctx, "kube-system", "harvester-load-balancer", client, func(ctx context.Context) {
		// register controllers of v1beta1 version
		if err := ippool.Register(ctx, management); err != nil {
			logrus.Fatalf("error register ip pool controller: %s", err.Error())
		}
		if err := loadbalancer.Register(ctx, management); err != nil {
			logrus.Fatalf("error register loadBalancer controller: %s", err.Error())
		}
		if err := vmi.Register(ctx, management); err != nil {
			logrus.Fatalf("error register vmi controller: %s", err.Error())
		}
		// if err := crd.Register(ctx, management); err != nil {
		// 	logrus.Fatalf("error register crd controller: %s", err.Error())
		// }
		// Start all the controllers
		if err := management.Start(2); err != nil {
			logrus.Fatalf("error starting: %s", err.Error())
		}
	})

	<-ctx.Done()
}
