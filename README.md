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

## License
Copyright (c) 2025 [SUSE, LLC.](https://www.suse.com/)

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

