package main

import (
	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/discovery/v1beta1"

	"github.com/harvester/harvester-conveyor/pkg/apis/network.harvesterhci.io/v1alpha1"
)

func main() {
	controllergen.Run(args.Options{
		OutputPackage: "github.com/harvester/harvester-conveyor/pkg/generated",
		Boilerplate:   "hack/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"network.harvesterhci.io": {
				Types: []interface{}{
					v1alpha1.LoadBalancer{},
				},
				GenerateTypes:   true,
				GenerateClients: true,
			},
			"": {
				Types: []interface{}{
					v1.Service{},
					v1beta1.EndpointSlice{},
				},
				InformersPackage: "k8s.io/client-go/informers",
				ClientSetPackage: "k8s.io/client-go/kubernetes",
				ListersPackage:   "k8s.io/client-go/listers",
			},
		},
	})
}
