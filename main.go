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
	"github.com/urfave/cli"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/harvester/harvester-load-balancer/pkg/config"
	"github.com/harvester/harvester-load-balancer/pkg/controller/ippool"
	"github.com/harvester/harvester-load-balancer/pkg/controller/loadbalancer"
	"github.com/harvester/harvester-load-balancer/pkg/controller/vmi"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const name = "harvester-load-balancer"

var (
	threadiness int
	logLevel    string
	VERSION     string // injected by linkflag
)

func main() {
	flags := []cli.Flag{
		cli.StringFlag{
			Name:        "loglevel",
			Usage:       "Specify log level",
			EnvVar:      "LOGLEVEL",
			Value:       "info",
			Destination: &logLevel,
		},
		cli.IntFlag{
			Name:        "threadiness",
			EnvVar:      "THREADINESS",
			Usage:       "Specify controller threads",
			Value:       2,
			Destination: &threadiness,
		},
	}

	logrus.Infof("Starting %v version %v", name, VERSION)

	cfg, err := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG")).ClientConfig()
	if err != nil {
		logrus.Fatal(err)
	}

	ctx := signals.SetupSignalContext()

	app := cli.NewApp()
	app.Flags = flags
	app.Action = func(c *cli.Context) {
		utils.SetLogLevel(logLevel)
		run(ctx, cfg)
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatalf("run harvester load balancer controller failed: %v", err)
	}
}

func run(ctx context.Context, cfg *rest.Config) {
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
		// Start all the controllers
		if err := management.Start(threadiness); err != nil {
			logrus.Fatalf("error starting: %s", err.Error())
		}
	})

	<-ctx.Done()
}
