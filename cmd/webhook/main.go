package main

import (
	"context"
	"fmt"
	"os"

	ctlcni "github.com/harvester/harvester/pkg/generated/controllers/k8s.cni.cncf.io"
	ctlkubevirt "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io"
	"github.com/harvester/webhook/pkg/config"
	"github.com/harvester/webhook/pkg/server"
	ctlcore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/client-go/rest"

	ctllb "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
	"github.com/harvester/harvester-load-balancer/pkg/webhook/ippool"
	"github.com/harvester/harvester-load-balancer/pkg/webhook/loadbalancer"
)

const name = "harvester-load-balancer-webhook"

var VERSION string // injected by linkflag

func main() {
	var options config.Options
	var logLevel string

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
			Destination: &options.Threadiness,
		},
		cli.IntFlag{
			Name:        "https-port",
			EnvVar:      "WEBHOOK_SERVER_HTTPS_PORT",
			Usage:       "HTTPS listen port",
			Value:       8443,
			Destination: &options.HTTPSListenPort,
		},
		cli.StringFlag{
			Name:        "namespace",
			EnvVar:      "NAMESPACE",
			Destination: &options.Namespace,
			Usage:       "The harvester namespace",
			Value:       "harvester-system",
		},
		cli.StringFlag{
			Name:        "controller-user",
			EnvVar:      "CONTROLLER_USER_NAME",
			Destination: &options.ControllerUsername,
			Value:       "harvester-load-balancer-webhook",
			Usage:       "The harvester controller username",
		},
		cli.StringFlag{
			Name:        "gc-user",
			EnvVar:      "GARBAGE_COLLECTION_USER_NAME",
			Destination: &options.GarbageCollectionUsername,
			Usage:       "The system username that performs garbage collection",
			Value:       "system:serviceaccount:kube-system:generic-garbage-collector",
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
		if err := run(ctx, cfg, &options); err != nil {
			logrus.Fatalf("run webhook server failed: %v", err)
		}
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatalf("run webhook server failed: %v", err)
	}
}

func run(ctx context.Context, cfg *rest.Config, options *config.Options) error {
	cniFactory := ctlcni.NewFactoryFromConfigOrDie(cfg)
	lbFactory := ctllb.NewFactoryFromConfigOrDie(cfg)
	kubevirtFactory := ctlkubevirt.NewFactoryFromConfigOrDie(cfg)
	coreFactory := ctlcore.NewFactoryFromConfigOrDie(cfg)

	// must declare before start the nad factory
	poolCache := lbFactory.Loadbalancer().V1beta1().IPPool().Cache()
	vmiCache := kubevirtFactory.Kubevirt().V1().VirtualMachineInstance().Cache()
	nadCache := cniFactory.K8s().V1().NetworkAttachmentDefinition().Cache()
	namespaceCache := coreFactory.Core().V1().Namespace().Cache()

	if err := start.All(ctx, options.Threadiness,
		cniFactory, lbFactory, kubevirtFactory, coreFactory); err != nil {
		return fmt.Errorf("failed to start controller: %w", err)
	}

	webhookServer := server.NewWebhookServer(ctx, cfg, name, options)

	if err := webhookServer.RegisterValidators(ippool.NewIPPoolValidator(poolCache),
		loadbalancer.NewValidator()); err != nil {
		return fmt.Errorf("failed to register ip pool validator: %w", err)
	}

	if err := webhookServer.RegisterMutators(ippool.NewIPPoolMutator(nadCache),
		loadbalancer.NewMutator(namespaceCache, vmiCache)); err != nil {
		return fmt.Errorf("failed to register ip pool mutator: %w", err)
	}

	if err := webhookServer.RegisterConverters(loadbalancer.NewConverter(vmiCache, poolCache)); err != nil {
		return fmt.Errorf("failed to register load balancer converter: %w", err)
	}

	if err := webhookServer.Start(); err != nil {
		return fmt.Errorf("failed to start webhook server: %w", err)
	}

	<-ctx.Done()

	return nil
}
