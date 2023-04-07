package store

import (
	"net"

	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

// FakeStore for testdata
type FakeStore struct {
	pool *lbv1.IPPool
}

// Store implements the Store interface
var _ backend.Store = &Store{}

func NewFakeStore(name string, ranges []lbv1.Range) *FakeStore {
	return &FakeStore{pool: &lbv1.IPPool{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec:       lbv1.IPPoolSpec{Ranges: ranges},
	}}
}

func (f *FakeStore) Lock() error {
	return nil
}

func (f *FakeStore) Unlock() error {
	return nil
}

func (f *FakeStore) Close() error {
	return nil
}

func (f *FakeStore) Reserve(applicantID, _ string, ip net.IP, _ string) (bool, error) {
	ipStr := ip.String()

	if f.pool.Status.Allocated != nil {
		if _, ok := f.pool.Status.Allocated[ipStr]; ok {
			return false, nil
		}
	}

	if f.pool.Status.AllocatedHistory != nil {
		delete(f.pool.Status.AllocatedHistory, ipStr)
	}
	if f.pool.Status.Allocated == nil {
		f.pool.Status.Allocated = make(map[string]string)
	}

	f.pool.Status.Allocated[ipStr] = applicantID
	f.pool.Status.LastAllocated = ipStr
	f.pool.Status.Available--

	return true, nil
}

func (f *FakeStore) LastReservedIP(_ string) (net.IP, error) {
	return net.ParseIP(f.pool.Status.LastAllocated), nil
}

func (f *FakeStore) Release(ip net.IP) error {
	if f.pool.Status.Allocated == nil {
		return nil
	}

	ipStr := ip.String()
	if f.pool.Status.AllocatedHistory == nil {
		f.pool.Status.AllocatedHistory = make(map[string]string)
	}
	f.pool.Status.AllocatedHistory[ipStr] = f.pool.Status.Allocated[ipStr]
	delete(f.pool.Status.Allocated, ipStr)
	f.pool.Status.Available++

	return nil
}

func (f *FakeStore) ReleaseByID(applicantID, _ string) error {
	if f.pool.Status.Allocated == nil {
		return nil
	}

	for ip, applicant := range f.pool.Status.Allocated {
		if applicant == applicantID {
			if f.pool.Status.AllocatedHistory == nil {
				f.pool.Status.AllocatedHistory = make(map[string]string)
			}
			f.pool.Status.AllocatedHistory[ip] = applicant
			delete(f.pool.Status.Allocated, ip)
			f.pool.Status.Available++
		}
	}

	return nil
}

func (f *FakeStore) GetByID(applicantID, _ string) []net.IP {
	if f.pool.Status.Allocated == nil {
		return nil
	}

	ips := make([]net.IP, 0, 10)

	for ip, applicant := range f.pool.Status.Allocated {
		if applicantID == applicant {
			ips = append(ips, net.ParseIP(ip))
		}
	}

	return ips
}
