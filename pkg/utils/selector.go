package utils

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func NewSelector(selector map[string][]string) (labels.Selector, error) {
	s := labels.NewSelector()
	requirements := make([]labels.Requirement, 0)
	for key, values := range selector {
		req, err := labels.NewRequirement(key, selection.In, values)
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, *req)
	}
	return s.Add(requirements...), nil
}

// the selector to match creator
func NewGuestClusterCreatorSelector() labels.Selector {
	return labels.Set(map[string]string{
		LabelKeyHarvesterCreator: GuestClusterHarvesterNodeDriver,
	}).AsSelector()
}

// the selector to match cluster name
func NewGuestClusterNameSelector(gcName string) labels.Selector {
	return labels.Set(map[string]string{
		LabelKeyGuestClusterNameOnVM: gcName,
	}).AsSelector()
}

// the selector to match both creator and cluster name
func NewGuestClusterNameAndCreatorNameSelector(gcName string) labels.Selector {
	return labels.Set(map[string]string{
		LabelKeyHarvesterCreator:     GuestClusterHarvesterNodeDriver,
		LabelKeyGuestClusterNameOnVM: gcName,
	}).AsSelector()
}
