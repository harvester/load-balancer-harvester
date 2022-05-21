package prober

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
)

type updateCondition func(workerKey string, isHealthy bool) error

type Manager struct {
	workers       map[string]*Worker
	workerLock    sync.RWMutex
	conditionChan chan healthCondition
	tcpProber     *tcpProber
}

func NewManager(ctx context.Context, handler updateCondition) *Manager {
	m := &Manager{
		workers:       make(map[string]*Worker),
		workerLock:    sync.RWMutex{},
		conditionChan: make(chan healthCondition),
		tcpProber:     newTCPProber(ctx),
	}

	go func() {
		for cond := range m.conditionChan {
			if err := handler(cond.workerUID, cond.isHealth); err != nil {
				logrus.Errorf("update status failed, key: %s, condition: %t", cond.workerUID, cond.isHealth)
			}
		}
	}()

	return m
}

func (m *Manager) GetWorker(uid string) (*Worker, bool) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	if w, ok := m.workers[uid]; ok {
		return w, true
	}
	return nil, false
}

func (m *Manager) ListWorkers() map[string]*Worker {
	return m.workers
}

func (m *Manager) AddWorker(uid string, option HealthOption) {
	w, existed := m.GetWorker(uid)
	if existed {
		if isChanged(option, w) {
			m.RemoveWorker(uid)
		} else {
			return
		}
	}

	logrus.Infof("add worker, uid: %s, option: %+v", uid, option)
	w = newWorker(uid, m.tcpProber, option, m.conditionChan)
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	m.workers[uid] = w
	go w.run()
}

func (m *Manager) RemoveWorker(uid string) {
	w, existed := m.GetWorker(uid)
	if !existed {
		return
	}
	logrus.Infof("remove worker, uid: %s", uid)
	w.stop()
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	delete(m.workers, uid)
}

func isChanged(o HealthOption, w *Worker) bool {
	if o.Address == w.address && o.Timeout == w.timeout && o.Period == w.Period &&
		o.SuccessThreshold == w.successThreshold && o.FailureThreshold == w.failureThreshold {
		return false
	}

	return true
}
