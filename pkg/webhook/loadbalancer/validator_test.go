package loadbalancer

import (
	"testing"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
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

	for _, tt := range tests {
		if err := checkListeners(tt.lb); (err != nil) != tt.wantErr {
			t.Errorf("%q. checkPorts() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
