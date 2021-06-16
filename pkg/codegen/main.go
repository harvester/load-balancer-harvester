package main

import (
	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
	"k8s.io/api/discovery/v1beta1"

	"github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
)

func main() {
	controllergen.Run(args.Options{
		OutputPackage: "github.com/harvester/harvester-load-balancer/pkg/generated",
		Boilerplate:   "hack/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"loadbalancer.harvesterhci.io": {
				Types: []interface{}{
					v1alpha1.LoadBalancer{},
				},
				GenerateTypes:   true,
				GenerateClients: true,
			},
			"discovery.k8s.io": {
				Types: []interface{}{
					v1beta1.EndpointSlice{},
				},
				GenerateClients: true,
			},
		},
	})
}
