package prober

import "time"

type HealthOption struct {
	Address          string
	SuccessThreshold int
	FailureThreshold int
	Timeout          time.Duration
	Period           time.Duration
	InitialCondition bool
}

type healthCondition struct {
	workerUID string
	isHealth  bool
}
