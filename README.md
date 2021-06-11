Harvester Load Balancer
==========================

Harvester Load Balancer is a load balancing controller. Combined with other components like Kube-vip and Harvester CCM, it makes Harvester a cloud provider.
Users can deploy Harvester Load Balancer in any other Kubernetes clusters without dependency on Harvester.

## Deploying
```
# helm v3
helm install deploy/harvester-load-balancer -n harvester-system
```

## Current status
This is in the pre-alpha stage of the development.

## License
Copyright (c) 2020 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

