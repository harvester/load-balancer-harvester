package main

import (
	"context"
	ctllb "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io"
	"github.com/rancher/wrangler/pkg/start"
	"os"

	ctlcni "github.com/harvester/harvester/pkg/generated/controllers/k8s.cni.cncf.io"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yaocw2020/webhook/pkg/config"
	"github.com/yaocw2020/webhook/pkg/server"
	"github.com/yaocw2020/webhook/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/harvester/harvester-load-balancer/pkg/webhook/ippool"
)

const name = "harvester-load-balancer-webhook"

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

	cfg, err := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG")).ClientConfig()
	if err != nil {
		logrus.Fatal(err)
	}

	client := kubernetes.NewForConfigOrDie(cfg)
	ctx := signals.SetupSignalContext()

	app := cli.NewApp()
	app.Flags = flags
	app.Action = func(c *cli.Context) {
		setLogLevel(logLevel)
		if err := run(ctx, cfg, &options); err != nil {
			logrus.Fatalf("run webhook server failed: %v", err)
		}
	}

	leader.RunOrDie(ctx, options.Namespace, name, client, func(ctx context.Context) {
		if err := app.Run(os.Args); err != nil {
			logrus.Fatalf("run webhook server failed: %v", err)
		}
	})
}

func run(ctx context.Context, cfg *rest.Config, options *config.Options) error {
	cniFactory := ctlcni.NewFactoryFromConfigOrDie(cfg)
	lbFacory := ctllb.NewFactoryFromConfigOrDie(cfg)
	// must declare before start the nad factory
	nadCache := cniFactory.K8s().V1().NetworkAttachmentDefinition().Cache()
	poolCache := lbFacory.Loadbalancer().V1alpha1().IPPool().Cache()
	if err := start.All(ctx, options.Threadiness, cniFactory, lbFacory); err != nil {
		return err
	}

	webhookServer := server.New(ctx, cfg, name, options)
	admitters := []types.Admitter{
		types.Validator2Admitter(ippool.NewIPPoolValidator(poolCache)),
		ippool.NewIPPoolMutator(nadCache),
	}
	webhookServer.Register(admitters)
	if err := webhookServer.Start(); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}

func setLogLevel(level string) {
	ll, err := logrus.ParseLevel(level)
	if err != nil {
		ll = logrus.DebugLevel
	}
	// set global log level
	logrus.SetLevel(ll)
}
