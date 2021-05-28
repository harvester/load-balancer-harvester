package entry

import (
	ctlCorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
)

const KeyLabelEntry = "loadbalancer.harvesterhci.io/entry"

type Service struct {
	client ctlCorev1.ServiceClient
	config *lbv1.LoadBalancer
}

func NewService(client *ctlCorev1.ServiceClient, config *lbv1.LoadBalancer) *Service {
	return &Service{
		client: *client,
		config: config,
	}
}

func (s *Service) Ensure() error {
	svc, err := s.client.Get(s.config.Namespace, s.config.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if err == nil {
		svc = s.construct(svc)
		_, err = s.client.Update(svc.DeepCopy())
		return err
	} else {
		svc = s.construct(nil)
		_, err := s.client.Create(svc)
		return err
	}
}

func (s *Service) construct(cur *corev1.Service) *corev1.Service {
	svc := &corev1.Service{}
	if cur != nil {
		svc = cur
	} else {
		svc.Name = s.config.Name
		svc.Namespace = s.config.Namespace
		svc.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: s.config.APIVersion,
			Kind:       s.config.Kind,
			Name:       s.config.Name,
			UID:        s.config.UID,
		}}
		svc.Labels = map[string]string{
			KeyLabelEntry: "true",
		}
	}

	if s.config.Spec.Type == lbv1.Internal {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
	} else {
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			// Kube-vip gets external IP for service of type LoadBalancer by DHCP if LoadBalancerIP is set as "0.0.0.0"
			svc.Spec.LoadBalancerIP = "0.0.0.0"
		}
	}

	ports := []corev1.ServicePort{}
	for _, listener := range s.config.Spec.Listeners {
		port := corev1.ServicePort{
			Name:       listener.Name,
			Protocol:   listener.Protocol,
			Port:       listener.Port,
			TargetPort: intstr.IntOrString{IntVal: listener.BackendPort},
		}
		ports = append(ports, port)
	}
	svc.Spec.Ports = ports

	return svc
}
