Harvester Load Balancer
==========================
[![Build Status](https://drone-publish.rancher.io/api/badges/harvester/load-balancer-harvester/status.svg)](https://drone-publish.rancher.io/harvester/load-balancer-harvester)
[![Go Report Card](https://goreportcard.com/badge/github.com/harvester/load-balancer-harvester)](https://goreportcard.com/report/github.com/harvester/load-balancer-harvester)
[![Releases](https://img.shields.io/github/release/harvester/load-balancer-harvester/all.svg)](https://github.com/harvester/load-balancer-harvester/releases)

Harvester Load Balancer is a load balancing controller. Combined with other components like [kube-vip](https://github.com/kube-vip/kube-vip) and [Harvester CCM](https://github.com/harvester/cloud-provider-harvester), it makes Harvester a cloud provider.
Users can deploy Harvester Load Balancer in any other Kubernetes clusters without dependency on Harvester.

## Deploying

```
# helm v3
$ helm repo add harvester https://charts.harvesterhci.io
$ helm repo add kube-vip https://kube-vip.io/helm-charts
$ helm repo update
$ helm install harvester-load-balancer harvester/harvester-load-balancer -n harvester-system
$ helm install kube-vip kube-vip/kube-vip --set config.vip_interface=<network interface> -n harvester-system 
$ helm install kube-vip/kube-vip-cloud-provider 
```

:::

Note: The `load-balancer-harvester` assumes that it is running in Harvester clusters. The best practice is to [install](https://docs.harvesterhci.io/v1.6/install/index) a Harvester cluster, it rolls out everything automatically.

:::

## How to Contribute

General guide is on [Harvester Developer Guide](https://github.com/harvester/harvester/blob/master/DEVELOPER_GUIDE.md).

### Build

1. Run `make ci` on the source code

```
/go/src/github.com/harvester/load-balancer-harvester$ make ci
```

1. A successful run will generate following container images.

```
REPOSITORY                                                                                           TAG                                         IMAGE ID       CREATED         SIZE
rancher/harvester-load-balancer                                                                      65c19c0e-amd64                              0dff190c91f5   2 hours ago     101MB
rancher/harvester-load-balancer-webhook                                                              d7cb2768-amd64                              7eae3c27859b   2 hours ago     104MB
```

1. Push or upload the new images to the running cluster, replace them to the deployments and test your change.

### Add new CRDs

Run `go generate` to generate related codes.

```
/go/src/github.com/harvester/load-balancer-harvester$ go generate
```

### Chart

The chart definition is managed on a central repo `https://github.com/harvester/charts`. Changes need to be sent to it.

https://github.com/harvester/charts/tree/master/charts/harvester-load-balancer

For more information, see [Chart README](https://github.com/harvester/charts/blob/master/README.md).

## License

Copyright (c) 2025 [SUSE, LLC.](https://www.suse.com/)

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

