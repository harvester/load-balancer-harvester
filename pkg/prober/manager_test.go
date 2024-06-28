package prober

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

const (
	healthyCase   = "healthy case"
	unhealthyCase = "unhealthy case"

	// dns.opendns.com
	healthyAddress   = "208.67.222.222:80"
	unhealthyAddress = "xxxxxxxxxx:80"
)

func TestManager(t *testing.T) {
	if err := mng.AddWorker(healthyCase, healthyAddress, HealthOption{
		Address:          healthyAddress,
		SuccessThreshold: 1,
		FailureThreshold: 3,
		Timeout:          time.Second,
		Period:           time.Second,
		InitialCondition: false,
	}); err != nil {
		t.Errorf("case: %s, add worker failed %s", healthyCase, err.Error())
	}
	time.Sleep(time.Second * 2)
	if len(mng.workers) == 0 {
		t.Errorf("case: %s, add worker failed", healthyCase)
	}
	if len(mng.workers[healthyCase]) == 0 {
		t.Errorf("case: %s, add worker address %s failed", healthyCase, healthyAddress)
	}
	if _, ok := mng.workers[healthyCase][healthyAddress]; !ok {
		t.Errorf("case: %s, works map is wrong", healthyCase)
	}
	if !mng.workers[healthyCase][healthyAddress].condition {
		t.Errorf("it should be able to connect %s", healthyAddress)
	}
	if _, err := mng.RemoveWorker(healthyCase, healthyAddress); err != nil {
		t.Errorf("case: %s, remove worker failed %s", healthyCase, err.Error())
	}
	if len(mng.workers[healthyCase]) != 0 {
		t.Errorf("case: %s, remove worker failed", healthyCase)
	}

	if err := mng.AddWorker(unhealthyCase, unhealthyAddress, HealthOption{
		Address:          unhealthyAddress,
		SuccessThreshold: 1,
		FailureThreshold: 2,
		Timeout:          time.Second,
		Period:           time.Second,
		InitialCondition: true,
	}); err != nil {
		t.Errorf("case: %s, add worker failed %s", unhealthyCase, err.Error())
	}
	if len(mng.workers[unhealthyCase]) != 1 {
		t.Errorf("case: %s, Add worker failed, len=%d", unhealthyCase, len(mng.workers[unhealthyCase]))
	}
	time.Sleep(time.Second * 5)
	if mng.workers[unhealthyCase][unhealthyAddress].condition {
		t.Errorf("it should not be able to connect %s", unhealthyAddress)
	}
	if _, err := mng.RemoveWorker(unhealthyCase, unhealthyAddress); err != nil {
		t.Errorf("case: %s, remove worker failed %s", unhealthyCase, err.Error())
	}
	if len(mng.workers[unhealthyAddress]) != 0 {
		t.Errorf("case: %s, remove worker failed", unhealthyCase)
	}
}

var mng *Manager

func TestMain(m *testing.M) {
	mng = NewManager(context.TODO(), printCondition)
	code := m.Run()
	os.Exit(code)
}

func printCondition(uid, address string, isHealthy bool) error {
	fmt.Printf("health check result, uid: %s, address %s, isHealthy: %t\n", uid, address, isHealthy)
	return nil
}
