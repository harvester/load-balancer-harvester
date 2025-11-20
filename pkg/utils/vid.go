package utils

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	ctlcniv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/k8s.cni.cncf.io/v1"

	lb "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io"
)

const (
	KeyVid       = lb.GroupName + "/vid"
	KeyVlanLabel = "network.harvesterhci.io/vlan-id"
)

// GetVid from the network attachment definition
func GetVid(network string, nadCache ctlcniv1.NetworkAttachmentDefinitionCache) (int, error) {
	if network == "" {
		return 0, nil
	}

	fields := strings.Split(network, "/")
	if len(fields) != 2 {
		return 0, fmt.Errorf("invalid network %s", network)
	}
	nad, err := nadCache.Get(fields[0], fields[1])
	if err != nil {
		return 0, err
	}

	// get the vid from the label network.harvesterhci.io/vlan-id
	if vlanStr, ok := nad.Labels[KeyVlanLabel]; ok {
		vid, err := strconv.Atoi(vlanStr)
		if err != nil {
			return 0, fmt.Errorf("invalid vlan %s", vlanStr)
		}
		return vid, nil
	}

	// Or get the vid from nad.Spec.Config
	netConf := &struct {
		VLAN int `json:"vlan"`
	}{}
	if err := json.Unmarshal([]byte(nad.Spec.Config), netConf); err != nil {
		return 0, err
	}
	return netConf.VLAN, nil
}

// input format: "192.168.5.12: default/cluster1-lb-3"
func SplitIPAllocatedString(ipStr string) (ip, namespace, name string, err error) {
	ipStr = strings.Trim(ipStr, " ")
	fields := strings.Split(ipStr, ":")
	if len(fields) != 2 {
		err = fmt.Errorf("%s is not a valid allocation record", ipStr)
		return
	}
	nsnameStr := strings.Trim(fields[1], " ")
	fields2 := strings.Split(nsnameStr, "/")
	if len(fields2) != 2 {
		err = fmt.Errorf("%s is not a valid allocation record", ipStr)
		return
	}
	ip = fields[0]
	namespace = fields2[0]
	name = fields2[1]
	err = nil
	return
}
