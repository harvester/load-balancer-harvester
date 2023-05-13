package store

import (
	"fmt"
	"net"

	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend"

	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
)

// Store is a CRD store that records the allocated IP in the status of IPPool CR
type Store struct {
	iPPoolName   string
	iPPoolCache  ctllbv1.IPPoolCache
	iPPoolClient ctllbv1.IPPoolClient
}

// Store implements the Store interface
var _ backend.Store = &Store{}

func New(ipPoolName string, ipPoolCache ctllbv1.IPPoolCache, ipPoolClient ctllbv1.IPPoolClient) *Store {
	return &Store{
		iPPoolName:   ipPoolName,
		iPPoolCache:  ipPoolCache,
		iPPoolClient: ipPoolClient,
	}
}

func (s *Store) Lock() error {
	return nil
}

func (s *Store) Unlock() error {
	return nil
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) Reserve(applicantID, _ string, ip net.IP, _ string) (bool, error) {
	ipPool, err := s.iPPoolCache.Get(s.iPPoolName)
	if err != nil {
		return false, err
	}

	ipStr := ip.String()

	// return false if the ip has been allocated
	if ipPool.Status.Allocated != nil {
		if _, ok := ipPool.Status.Allocated[ipStr]; ok {
			return false, nil
		}
	}

	ipPoolCopy := ipPool.DeepCopy()
	if ipPoolCopy.Status.AllocatedHistory != nil {
		delete(ipPoolCopy.Status.AllocatedHistory, ipStr)
	}
	if ipPoolCopy.Status.Allocated == nil {
		ipPoolCopy.Status.Allocated = make(map[string]string)
	}
	ipPoolCopy.Status.Allocated[ipStr] = applicantID
	ipPoolCopy.Status.LastAllocated = ipStr
	ipPoolCopy.Status.Available--

	if _, err := s.iPPoolClient.Update(ipPoolCopy); err != nil {
		return false, fmt.Errorf("record %s into %s failed", ipStr, s.iPPoolName)
	}

	return true, nil
}

func (s *Store) LastReservedIP(_ string) (net.IP, error) {
	ipPool, err := s.iPPoolCache.Get(s.iPPoolName)
	if err != nil {
		return nil, err
	}

	return net.ParseIP(ipPool.Status.LastAllocated), nil
}

func (s *Store) Release(ip net.IP) error {
	ipPool, err := s.iPPoolCache.Get(s.iPPoolName)
	if err != nil {
		return err
	}
	if ipPool.Status.Allocated == nil {
		return nil
	}

	ipPoolCopy := ipPool.DeepCopy()
	ipStr := ip.String()

	if ipPool.Status.AllocatedHistory == nil {
		ipPoolCopy.Status.AllocatedHistory = make(map[string]string)
	}
	ipPoolCopy.Status.AllocatedHistory[ipStr] = ipPool.Status.Allocated[ipStr]
	delete(ipPoolCopy.Status.Allocated, ipStr)
	ipPoolCopy.Status.Available++

	_, err = s.iPPoolClient.Update(ipPoolCopy)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) ReleaseByID(applicantID, _ string) error {
	ipPool, err := s.iPPoolCache.Get(s.iPPoolName)
	if err != nil {
		return err
	}
	if ipPool.Status.Allocated == nil {
		return nil
	}

	ipPoolCopy := ipPool.DeepCopy()

	for ip, applicant := range ipPool.Status.Allocated {
		if applicant == applicantID {
			if ipPool.Status.AllocatedHistory == nil {
				ipPoolCopy.Status.AllocatedHistory = make(map[string]string)
			}
			ipPoolCopy.Status.AllocatedHistory[ip] = applicant
			delete(ipPoolCopy.Status.Allocated, ip)
			ipPoolCopy.Status.Available++
		}
	}

	_, err = s.iPPoolClient.Update(ipPoolCopy)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) GetByID(applicantID, _ string) []net.IP {
	ipPool, err := s.iPPoolCache.Get(s.iPPoolName)
	if err != nil || ipPool.Status.Allocated == nil {
		return nil
	}

	ips := make([]net.IP, 0, 10)

	for ip, applicant := range ipPool.Status.Allocated {
		if applicantID == applicant {
			ips = append(ips, net.ParseIP(ip))
		}
	}

	return ips
}
