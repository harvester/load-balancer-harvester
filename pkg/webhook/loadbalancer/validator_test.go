package loadbalancer

import (
	"strings"
	"testing"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func TestCheckListeners(t *testing.T) {
	tests := []struct {
		name     string
		lb       *lbv1.LoadBalancer
		wantErr  bool
		errorKey string
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
						{Name: "a", Port: -1, BackendPort: 80},
					},
				},
			},
			wantErr:  true,
			errorKey: "listener port",
		},
		{
			name: "port > 65535",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "b", Port: 800000, BackendPort: 80},
					},
				},
			},
			wantErr:  true,
			errorKey: "listener port",
		},
		{
			name: "backend port < 1",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", Port: 80, BackendPort: 0},
					},
				},
			},
			wantErr:  true,
			errorKey: "backend port",
		},
		{
			name: "backend port > 65535",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", Port: 80, BackendPort: 65536},
					},
				},
			},
			wantErr:  true,
			errorKey: "backend port",
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
		{
			name: "VM-type lb should have listeners defined",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			wantErr: true,
		},
		{
			name: "cluster-type lb can have no listeners",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
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
		{
			name: "Cluster type LB may set invalid health check, but it is skipped",
			lb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 32},
				},
			},
			wantErr: false,
		},
	}

	testsIPAM := []struct {
		name    string
		oldLb   *lbv1.LoadBalancer
		newLb   *lbv1.LoadBalancer
		wantErr bool
	}{
		{
			name: "IPAM can't be changed from empty to DHCP",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: "", // defaults to lbv1.Pool
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.DHCP,
				},
			},
			wantErr: true,
		},
		{
			name: "IPAM can't be changed from Pool to DHCP",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.Pool,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.DHCP,
				},
			},
			wantErr: true,
		},
		{
			name: "IPAM can't be changed from DHCP to Pool",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.DHCP,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.Pool,
				},
			},
			wantErr: true,
		},
		{
			name: "IPAM can't be changed from DHCP to empty",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.DHCP,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: "",
				},
			},
			wantErr: true,
		},
		{
			name: "IPAM keeps DHCP",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.DHCP,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.DHCP,
				},
			},
			wantErr: false,
		},
		{
			name: "IPAM keeps Pool",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.Pool,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.Pool,
				},
			},
			wantErr: false,
		},
		{
			name: "IPAM keeps empty",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: "",
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: "",
				},
			},
			wantErr: false,
		},
		{
			name: "IPAM changes from Pool to empty",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.Pool,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: "",
				},
			},
			wantErr: false,
		},
		{
			name: "IPAM changes from empty to Pool",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: "",
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					IPAM: lbv1.Pool,
				},
			},
			wantErr: false,
		},
	}

	testsWorkloadType := []struct {
		name    string
		oldLb   *lbv1.LoadBalancer
		newLb   *lbv1.LoadBalancer
		wantErr bool
	}{
		{
			name: "WorkloadType can't be changed from empty to Cluster",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: "", // defaults to lbv1.VM
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			wantErr: true,
		},
		{
			name: "WorkloadType can't be changed from VM to Cluster",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			wantErr: true,
		},
		{
			name: "WorkloadType can't be changed from Cluster to VM",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			wantErr: true,
		},
		{
			name: "WorkloadType can't be changed from Cluster to empty",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: "",
				},
			},
			wantErr: true,
		},
		{
			name: "WorkloadType keeps Cluster",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			wantErr: false,
		},
		{
			name: "WorkloadType keeps VM",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			wantErr: false,
		},
		{
			name: "WorkloadType keeps empty",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: "",
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: "",
				},
			},
			wantErr: false,
		},
		{
			name: "WorkloadType changes from VM to empty",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: "",
				},
			},
			wantErr: false,
		},
		{
			name: "WorkloadType changes from empty to VM",
			oldLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: "",
				},
			},
			newLb: &lbv1.LoadBalancer{
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		err := checkListeners(tt.lb)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. checkListeners() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if tt.wantErr && tt.errorKey != "" && !strings.Contains(err.Error(), tt.errorKey) {
			t.Errorf("%q, the return error %v does not include the keyword '%s'", tt.name, err, tt.errorKey)
		}
	}

	for _, tt := range testsHealtyCheck {
		if err := checkHealthyCheck(tt.lb); (err != nil) != tt.wantErr {
			t.Errorf("%q. checkHealthyCheck() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}

	for _, tt := range testsIPAM {
		if err := checkIPAM(tt.oldLb, tt.newLb); (err != nil) != tt.wantErr {
			t.Errorf("%q. checkIPAM() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}

	for _, tt := range testsWorkloadType {
		if err := checkWorkloadType(tt.oldLb, tt.newLb); (err != nil) != tt.wantErr {
			t.Errorf("%q. checkWorkloadType() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
