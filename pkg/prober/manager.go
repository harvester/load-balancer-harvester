package prober

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
)

type updateCondition func(uid, address string, isHealthy bool) error

type WorkerMap map[string]*Worker

type Manager struct {
	workers       map[string]WorkerMap
	workerLock    sync.RWMutex
	conditionChan chan healthCondition
	tcpProber     *tcpProber
}

func NewManager(ctx context.Context, handler updateCondition) *Manager {
	m := &Manager{
		workers:       make(map[string]WorkerMap),
		workerLock:    sync.RWMutex{},
		conditionChan: make(chan healthCondition),
		tcpProber:     newTCPProber(ctx),
	}

	go func() {
		for cond := range m.conditionChan {
			if err := handler(cond.uid, cond.address, cond.isHealthy); err != nil {
				logrus.Errorf("prober update status to manager failed, uid:%s, address: %s, condition: %t", cond.uid, cond.address, cond.isHealthy)
			}
		}
	}()

	return m
}

func (m *Manager) GetWorkerHealthOptionMap(uid string) (map[string]HealthOption, error) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	if wm, ok := m.workers[uid]; ok {
		// copy out
		prob := make(map[string]HealthOption)
		for _, w := range wm {
			prob[w.Address] = w.HealthOption
		}
		return prob, nil
	}
	return nil, nil
}

func (m *Manager) AddWorker(uid string, address string, option HealthOption) error {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	wm, ok := m.workers[uid]
	if !ok {
		wm = make(WorkerMap)
		m.workers[uid] = wm
	}

	// stop if duplicated
	if w, ok := wm[address]; ok {
		logrus.Infof("porber worker already exists, uid %s, address %s, will stop it", uid, address)
		w.stop()
	}
	w := newWorker(uid, m.tcpProber, option, m.conditionChan)
	wm[address] = w

	go w.run()

	logrus.Infof("add porber worker, uid: %s, address: %s, option: %+v", uid, address, option)
	return nil
}

func (m *Manager) RemoveWorker(uid, address string) (int, error) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	cnt := 0
	if wm, ok := m.workers[uid]; ok {
		if w, ok := wm[address]; ok {
			w.stop()
			delete(wm, address)
			cnt = 1
		}
	}

	if cnt > 0 {
		logrus.Infof("remove porber worker, uid: %s, address: %s", uid, address)
	}

	return cnt, nil
}

func (m *Manager) RemoveWorkersByUid(uid string) (int, error) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	cnt := 0
	if wm, ok := m.workers[uid]; ok {
		cnt = len(wm)
		for k, w := range wm {
			w.stop()
			delete(wm, k)
		}
		delete(m.workers, uid)
	}

	if cnt > 0 {
		logrus.Infof("remove %d porber workers from uid: %s", cnt, uid)
	}

	return cnt, nil
}
