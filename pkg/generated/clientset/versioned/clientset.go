/*
Copyright 2019 Wrangler Sample Controller Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package versioned

import (
	"fmt"
	"net/http"

	discoveryv1 "github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/typed/discovery.k8s.io/v1"
	loadbalancerv1alpha1 "github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/typed/loadbalancer.harvesterhci.io/v1alpha1"
	loadbalancerv1beta1 "github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/typed/loadbalancer.harvesterhci.io/v1beta1"
	discovery "k8s.io/client-go/discovery"
	rest "k8s.io/client-go/rest"
	flowcontrol "k8s.io/client-go/util/flowcontrol"
)

type Interface interface {
	Discovery() discovery.DiscoveryInterface
	DiscoveryV1() discoveryv1.DiscoveryV1Interface
	LoadbalancerV1beta1() loadbalancerv1beta1.LoadbalancerV1beta1Interface
	LoadbalancerV1alpha1() loadbalancerv1alpha1.LoadbalancerV1alpha1Interface
}

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	*discovery.DiscoveryClient
	discoveryV1          *discoveryv1.DiscoveryV1Client
	loadbalancerV1beta1  *loadbalancerv1beta1.LoadbalancerV1beta1Client
	loadbalancerV1alpha1 *loadbalancerv1alpha1.LoadbalancerV1alpha1Client
}

// DiscoveryV1 retrieves the DiscoveryV1Client
func (c *Clientset) DiscoveryV1() discoveryv1.DiscoveryV1Interface {
	return c.discoveryV1
}

// LoadbalancerV1beta1 retrieves the LoadbalancerV1beta1Client
func (c *Clientset) LoadbalancerV1beta1() loadbalancerv1beta1.LoadbalancerV1beta1Interface {
	return c.loadbalancerV1beta1
}

// LoadbalancerV1alpha1 retrieves the LoadbalancerV1alpha1Client
func (c *Clientset) LoadbalancerV1alpha1() loadbalancerv1alpha1.LoadbalancerV1alpha1Interface {
	return c.loadbalancerV1alpha1
}

// Discovery retrieves the DiscoveryClient
func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	if c == nil {
		return nil
	}
	return c.DiscoveryClient
}

// NewForConfig creates a new Clientset for the given config.
// If config's RateLimiter is not set and QPS and Burst are acceptable,
// NewForConfig will generate a rate-limiter in configShallowCopy.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(c *rest.Config) (*Clientset, error) {
	configShallowCopy := *c

	if configShallowCopy.UserAgent == "" {
		configShallowCopy.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	// share the transport between all clients
	httpClient, err := rest.HTTPClientFor(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	return NewForConfigAndClient(&configShallowCopy, httpClient)
}

// NewForConfigAndClient creates a new Clientset for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
// If config's RateLimiter is not set and QPS and Burst are acceptable,
// NewForConfigAndClient will generate a rate-limiter in configShallowCopy.
func NewForConfigAndClient(c *rest.Config, httpClient *http.Client) (*Clientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		if configShallowCopy.Burst <= 0 {
			return nil, fmt.Errorf("burst is required to be greater than 0 when RateLimiter is not set and QPS is set to greater than 0")
		}
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}

	var cs Clientset
	var err error
	cs.discoveryV1, err = discoveryv1.NewForConfigAndClient(&configShallowCopy, httpClient)
	if err != nil {
		return nil, err
	}
	cs.loadbalancerV1beta1, err = loadbalancerv1beta1.NewForConfigAndClient(&configShallowCopy, httpClient)
	if err != nil {
		return nil, err
	}
	cs.loadbalancerV1alpha1, err = loadbalancerv1alpha1.NewForConfigAndClient(&configShallowCopy, httpClient)
	if err != nil {
		return nil, err
	}

	cs.DiscoveryClient, err = discovery.NewDiscoveryClientForConfigAndClient(&configShallowCopy, httpClient)
	if err != nil {
		return nil, err
	}
	return &cs, nil
}

// NewForConfigOrDie creates a new Clientset for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *Clientset {
	cs, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return cs
}

// New creates a new Clientset for the given RESTClient.
func New(c rest.Interface) *Clientset {
	var cs Clientset
	cs.discoveryV1 = discoveryv1.New(c)
	cs.loadbalancerV1beta1 = loadbalancerv1beta1.New(c)
	cs.loadbalancerV1alpha1 = loadbalancerv1alpha1.New(c)

	cs.DiscoveryClient = discovery.NewDiscoveryClient(c)
	return &cs
}
