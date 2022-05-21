package prober

import "time"

type HealthOption struct {
	Address          string
	SuccessThreshold uint
	FailureThreshold uint
	Timeout          time.Duration
	Period           time.Duration
	InitialCondition bool
}

type healthCondition struct {
	workerUID string
	isHealth  bool
}
