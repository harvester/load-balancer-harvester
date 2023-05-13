package servicelb

import (
	"net"

	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/harvester-load-balancer/pkg/lb"
)

type Server struct {
	*kubevirtv1.VirtualMachineInstance
}

var _ lb.BackendServer = &Server{}

func (s *Server) GetAddress() (string, bool) {
	for _, networkInterface := range s.Status.Interfaces {
		if ip := net.ParseIP(networkInterface.IP); ip.To4() != nil {
			return networkInterface.IP, true
		}
	}

	return "", false
}
