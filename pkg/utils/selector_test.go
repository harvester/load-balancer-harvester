package utils

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"
)

func TestNewGuestClusterCreatorSelector(t *testing.T) {
	selector := NewGuestClusterCreatorSelector()

	tests := []struct {
		name   string
		labels labels.Set
		want   bool
	}{
		{
			name: "match creator",
			labels: labels.Set{
				LabelKeyHarvesterCreator: GuestClusterHarvesterNodeDriver,
			},
			want: true,
		},
		{
			name: "does not match creator",
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

func TestNewGuestClusterNameSelector(t *testing.T) {
	clusterName := "test-cluster"
	selector := NewGuestClusterNameSelector(clusterName)

	tests := []struct {
		name   string
		labels labels.Set
		want   bool
	}{
		{
			name: "match cluster name",
			labels: labels.Set{
				LabelKeyGuestClusterNameOnVM: clusterName,
			},
			want: true,
		},
		{
			name: "does not match cluster name",
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

func TestNewGuestClusterNameAndCreatorNameSelector(t *testing.T) {
	clusterName := "test-cluster"
	selector := NewGuestClusterNameAndCreatorNameSelector(clusterName)

	tests := []struct {
		name   string
		labels labels.Set
		want   bool
	}{
		{
			name: "match cluster name and creator",
			labels: labels.Set{
				LabelKeyGuestClusterNameOnVM: clusterName,
				LabelKeyHarvesterCreator:     GuestClusterHarvesterNodeDriver,
			},
			want: true,
		},
		{
			name: "does not match cluster name",
			labels: labels.Set{
				LabelKeyGuestClusterNameOnVM: "other-cluster",
				LabelKeyHarvesterCreator:     GuestClusterHarvesterNodeDriver,
			},
			want: false,
		},
		{
			name: "does not match creator",
			labels: labels.Set{
				LabelKeyGuestClusterNameOnVM: clusterName,
				LabelKeyHarvesterCreator:     "other-driver",
			},
			want: false,
		},
		{
			name:   "does not match any",
			labels: labels.Set{},
			want:   false,
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
