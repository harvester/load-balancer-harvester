package prober

import (
	"time"

	"k8s.io/klog/v2"
)

type Worker struct {
	tcpProber        Prober
	uid              string
	address          string
	successThreshold int
	successCounter   int
	failureThreshold int
	failureCounter   int
	timeout          time.Duration
	Period           time.Duration
	condition        bool
	conditionChan    chan healthCondition
	stopCh           chan struct{}
}

func newWorker(uid string, tcpProber Prober, option HealthOption, conditionChan chan healthCondition) *Worker {
	return &Worker{
		tcpProber:        tcpProber,
		uid:              uid,
		address:          option.Address,
		successThreshold: option.SuccessThreshold,
		successCounter:   0,
		failureThreshold: option.FailureThreshold,
		failureCounter:   0,
		timeout:          option.Timeout,
		Period:           option.Period,
		condition:        option.InitialCondition,
		conditionChan:    conditionChan,
		stopCh:           make(chan struct{}),
	}
}

func (w *Worker) run() {
	ticker := time.NewTicker(w.Period)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.doProbe()
		}
	}
}

func (w *Worker) stop() {
	w.stopCh <- struct{}{}
}

// probe only supports TCP
func (w *Worker) probe() error {
	return w.tcpProber.Probe(w.address, w.timeout)
}

func (w *Worker) doProbe() {
	if err := w.probe(); err != nil {
		klog.V(4).Infof("probe error, %s, address: %s, timeout: %v", err.Error(), w.address, w.timeout)
		w.successCounter = 0
		w.failureCounter++
	} else {
		klog.V(4).Infof("probe successful, address: %s, timeout: %v", w.address, w.timeout)
		w.failureCounter = 0
		w.successCounter++
	}
	if w.successCounter == w.successThreshold {
		if !w.condition {
			w.condition = true
			w.conditionChan <- healthCondition{
				workerUID: w.uid,
				isHealth:  w.condition,
			}
		}
	}
	if w.failureCounter == w.failureThreshold {
		if w.condition {
			w.condition = false
			w.conditionChan <- healthCondition{
				workerUID: w.uid,
				isHealth:  w.condition,
			}
		}
	}
}
