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

// the selecotr to match creator
func NewGuestClusterCreatorSelecotr() labels.Selector {
	return labels.Set(map[string]string{
		LabelKeyHarvesterCreator: GuestClusterHarvesterNodeDriver,
	}).AsSelector()
}

// the selecotr to match clustername
func NewGuestClusterNameSelecotr(gcName string) labels.Selector {
	return labels.Set(map[string]string{
		LabelKeyGuestClusterNameOnVM: gcName,
	}).AsSelector()
}

func NewGuestClusterNameAndCreatorNameSelecotr(gcName string) labels.Selector {
	return labels.Set(map[string]string{
		LabelKeyHarvesterCreator:     GuestClusterHarvesterNodeDriver,
		LabelKeyGuestClusterNameOnVM: gcName,
	}).AsSelector()
}
