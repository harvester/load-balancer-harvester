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
	uid       string
	address   string
	isHealthy bool
}

func (ho *HealthOption) Equal(h HealthOption) bool {
	return ho.Address == h.Address && ho.SuccessThreshold == h.SuccessThreshold && ho.FailureThreshold == h.FailureThreshold &&
		ho.Timeout == h.Timeout && ho.Period == h.Period
}
