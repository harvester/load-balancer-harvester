package main

import (
	controllergen "github.com/rancher/wrangler/v3/pkg/controller-gen"
	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
	v1 "k8s.io/api/discovery/v1"

	lbv1alpha1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

func main() {
	controllergen.Run(args.Options{
		OutputPackage: "github.com/harvester/harvester-load-balancer/pkg/generated",
		Boilerplate:   "hack/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"loadbalancer.harvesterhci.io": {
				Types: []interface{}{
					lbv1.LoadBalancer{},
					lbv1.IPPool{},
					lbv1alpha1.LoadBalancer{},
				},
				GenerateTypes:   true,
				GenerateClients: true,
			},
			"discovery.k8s.io": {
				Types: []interface{}{
					v1.EndpointSlice{},
				},
				GenerateClients: true,
			},
		},
	})
}
