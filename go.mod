module github.com/harvester/harvester-conveyor

go 1.15

replace k8s.io/client-go => k8s.io/client-go v0.20.0

require (
	github.com/rancher/lasso v0.0.0-20210408231703-9ddd9378d08d
	github.com/rancher/wrangler v0.8.0
	github.com/sirupsen/logrus v1.4.2
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog/v2 v2.8.0
)
