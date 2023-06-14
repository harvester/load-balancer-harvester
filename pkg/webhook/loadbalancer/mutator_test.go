package loadbalancer

import (
	"path/filepath"
	"testing"

	harvesterfake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	harvesterfakeclients "github.com/harvester/harvester/pkg/util/fakeclients"
	corefake "k8s.io/client-go/kubernetes/fake"

	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const mutatorCaseDirectory = "./testdata/mutator/"

// TestFindProject tests the function findProject
func TestFindProject(t *testing.T) {
	namespaces, err := utils.ParseFromFile(filepath.Join(mutatorCaseDirectory + "namespace.yaml"))
	if err != nil {
		t.Error(err)
	}
	// create a new mutator
	coreclientset := corefake.NewSimpleClientset(namespaces...)
	m := &mutator{
		namespaceCache: harvesterfakeclients.NamespaceCache(coreclientset.CoreV1().Namespaces),
	}

	tests := []struct {
		namespace   string
		wantProject string
	}{
		{
			namespace:   "default",
			wantProject: "local/p-abcde",
		},
		{
			namespace:   "withoutProject",
			wantProject: "",
		},
	}

	for _, test := range tests {
		if project, err := m.findProject(test.namespace); err != nil {
			t.Error(err)
		} else if project != test.wantProject {
			t.Errorf("want project %s through namespace %s, got %s", test.wantProject, test.namespace, project)
		}
	}
}

// TestFindNetwork tests the function findNetwork
func TestFindNetwork(t *testing.T) {
	vmis, err := utils.ParseFromFile(filepath.Join(mutatorCaseDirectory + "vmi.yaml"))
	if err != nil {
		t.Error(err)
	}

	harvesterclientset := harvesterfake.NewSimpleClientset(vmis...)

	m := &mutator{
		vmiCache: harvesterfakeclients.VirtualMachineInstanceCache(harvesterclientset.KubevirtV1().VirtualMachineInstances),
	}

	tests := []struct {
		namespace       string
		clusterName     string
		wantNetworkName string
	}{
		{
			namespace:       "default",
			clusterName:     "rke2",
			wantNetworkName: "default/mgmt-untagged",
		},
	}

	for _, test := range tests {
		if network, err := m.findNetwork(test.namespace, test.clusterName); err != nil {
			t.Error(err)
		} else if network != test.wantNetworkName {
			t.Errorf("want network %s through namespace %s and cluster %s, got %s",
				test.wantNetworkName, test.namespace, test.clusterName, network)
		}
	}
}
