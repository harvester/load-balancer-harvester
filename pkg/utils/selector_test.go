package utils

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"
)

func Test_NewGuestClusterCreatorSelecotr(t *testing.T) {
	selector := NewGuestClusterCreatorSelecotr()

	tests := []struct {
		name   string
		labels labels.Set
		want   bool
	}{
		{
			name: "match correct creator",
			labels: labels.Set{
				LabelKeyHarvesterCreator: GuestClusterHarvesterNodeDriver,
			},
			want: true,
		},
		{
			name: "mismatch different creator",
			labels: labels.Set{
				LabelKeyHarvesterCreator: "manual-creation",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selector.Matches(tt.labels); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_NewGuestClusterNameSelecotr(t *testing.T) {
	clusterName := "test-cluster"
	selector := NewGuestClusterNameSelecotr(clusterName)

	tests := []struct {
		name   string
		labels labels.Set
		want   bool
	}{
		{
			name: "match correct cluster name",
			labels: labels.Set{
				LabelKeyGuestClusterNameOnVM: clusterName,
			},
			want: true,
		},
		{
			name: "mismatch wrong cluster name",
			labels: labels.Set{
				LabelKeyGuestClusterNameOnVM: "other-cluster",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selector.Matches(tt.labels); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_NewGuestClusterNameAndCreatorNameSelecotr(t *testing.T) {
	clusterName := "production-cluster"
	selector := NewGuestClusterNameAndCreatorNameSelecotr(clusterName)

	tests := []struct {
		name   string
		labels labels.Set
		want   bool
	}{
		{
			name: "match both labels correctly",
			labels: labels.Set{
				LabelKeyHarvesterCreator:     GuestClusterHarvesterNodeDriver,
				LabelKeyGuestClusterNameOnVM: clusterName,
			},
			want: true,
		},
		{
			name: "fail on incorrect creator",
			labels: labels.Set{
				LabelKeyHarvesterCreator:     "custom-driver",
				LabelKeyGuestClusterNameOnVM: clusterName,
			},
			want: false,
		},
		{
			name: "fail on incorrect name",
			labels: labels.Set{
				LabelKeyHarvesterCreator:     GuestClusterHarvesterNodeDriver,
				LabelKeyGuestClusterNameOnVM: "dev-cluster",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selector.Matches(tt.labels); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}
