module github.com/harvester/harvester-load-balancer

go 1.15

replace (
	github.com/rancher/wrangler => github.com/yaocw2020/wrangler v0.8.1-0.20210525085519-43f6d901819d
	k8s.io/client-go => k8s.io/client-go v0.20.0
)

require (
	github.com/rancher/lasso v0.0.0-20210408231703-9ddd9378d08d
	github.com/rancher/wrangler v0.8.0
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/sys v0.0.0-20210521203332-0cec03c779c1 // indirect
	golang.org/x/tools v0.1.1 // indirect
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.20.0 // indirect
	k8s.io/gengo v0.0.0-20210203185629-de9496dff47b // indirect
	k8s.io/klog/v2 v2.8.0
)
