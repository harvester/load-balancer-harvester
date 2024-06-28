package prober

import (
	"time"

	"github.com/sirupsen/logrus"
)

type Worker struct {
	HealthOption
	uid            string
	tcpProber      Prober
	successCounter uint
	failureCounter uint
	condition      bool
	conditionChan  chan healthCondition
	stopCh         chan struct{}
	logFailure     bool
	logSuccess     bool
}

func newWorker(uid string, tcpProber Prober, option HealthOption, conditionChan chan healthCondition) *Worker {
	return &Worker{
		tcpProber:      tcpProber,
		uid:            uid,
		HealthOption:   option,
		successCounter: 0,
		failureCounter: 0,
		conditionChan:  conditionChan,
		stopCh:         make(chan struct{}),
		condition:      option.InitialCondition,
		logFailure:     true,
		logSuccess:     true,
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
	return w.tcpProber.Probe(w.Address, w.Timeout)
}

func (w *Worker) doProbe() {
	// failure case
	if err := w.probe(); err != nil {
		w.successCounter = 0
		w.failureCounter++
		w.logSuccess = true
		if w.failureCounter >= w.FailureThreshold {
			// for continuous failure, only log error once in the controller life-cycle
			if w.logFailure {
				logrus.Infof("probe error uid:%s, address: %s, timeout: %v, error: %s", w.uid, w.Address, w.Timeout, err.Error())
				w.logFailure = false
			}
			// notify anyway, the receiver may fail when processing
			w.condition = false
			w.conditionChan <- healthCondition{
				uid:       w.uid,
				address:   w.Address,
				isHealthy: w.condition,
			}
			w.failureCounter = 0
		}
		return
	}

	// successful case
	w.failureCounter = 0
	w.successCounter++
	w.logFailure = true
	if w.successCounter >= w.SuccessThreshold {
		if w.logSuccess {
			logrus.Infof("probe successful, uid:%s, address: %s, timeout: %v", w.uid, w.Address, w.Timeout)
			w.logSuccess = false
		}
		// notify anyway, the receiver may fail when processing
		w.condition = true
		w.conditionChan <- healthCondition{
			uid:       w.uid,
			address:   w.Address,
			isHealthy: w.condition,
		}
		w.successCounter = 0
	}
}
