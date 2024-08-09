package loadbalancer

import (
	"testing"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func TestCheckListeners(t *testing.T) {
	tests := []struct {
		name    string
		lb      *lbv1.LoadBalancer
		wantErr bool
	}{
		{
			name: "duplicate name",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a"},
						{Name: "a"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate port",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", Port: 80},
						{Name: "b", Port: 80},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate backend port",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80},
						{Name: "b", BackendPort: 80},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "port < 1",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", Port: -1},
						{Name: "b", Port: 80},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "port > 65535",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", Port: 80},
						{Name: "b", Port: 8000},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "backend port < 1",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 0},
						{Name: "b", BackendPort: 80},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "backend port > 65535",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 65536},
						{Name: "b", BackendPort: 80},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "right case",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", Port: 80, BackendPort: 80},
						{Name: "b", Port: 81, BackendPort: 81},
					},
				},
			},
			wantErr: false,
		},
	}

	testsHealtyCheck := []struct {
		name    string
		lb      *lbv1.LoadBalancer
		wantErr bool
	}{
		{
			name: "health check port is not in backend port list",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 99},
				},
			},
			wantErr: true,
		},
		{
			name: "health check protocol is not expected tcp",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 32},
				},
			},
			wantErr: true,
		},
		{
			name: "health check parameter SuccessThreshold is error",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 80, SuccessThreshold: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "health check parameter FailureThreshold is error",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 80, FailureThreshold: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "health check parameter PeriodSeconds is error",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 80, PeriodSeconds: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "health check parameter TimeoutSeconds is error",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 80, TimeoutSeconds: 0},
				},
			},
			wantErr: true,
		},
		{
			name: "health check right case",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 80, SuccessThreshold: 1, FailureThreshold: 1, PeriodSeconds: 1, TimeoutSeconds: 1},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		if err := checkListeners(tt.lb); (err != nil) != tt.wantErr {
			t.Errorf("%q. checkListeners() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}

	for _, tt := range testsHealtyCheck {
		if err := checkHealthyCheck(tt.lb); (err != nil) != tt.wantErr {
			t.Errorf("%q. checkHealthyCheck() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
