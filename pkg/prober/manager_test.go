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

	healthyAddress   = "rancher.cn:80"
	unhealthyAddress = "xxxxxxxxxx:80"
)

func TestManager(t *testing.T) {
	mng.AddWorker(healthyCase, HealthOption{
		Address:          healthyAddress,
		SuccessThreshold: 1,
		FailureThreshold: 3,
		Timeout:          time.Second,
		Period:           time.Second,
		InitialCondition: false,
	})
	time.Sleep(time.Second * 2)
	if len(mng.workers) != 1 {
		t.Errorf("case; %s, add worker failed", healthyCase)
	}
	if !mng.workers[healthyCase].condition {
		t.Errorf("it should be able to connect %s", healthyAddress)
	}
	mng.RemoveWorker(healthyCase)
	if len(mng.workers) != 0 {
		t.Errorf("case: %s, remove worker failed", healthyCase)
	}

	mng.AddWorker(unhealthyCase, HealthOption{
		Address:          unhealthyAddress,
		SuccessThreshold: 1,
		FailureThreshold: 2,
		Timeout:          time.Second,
		Period:           time.Second,
		InitialCondition: true,
	})
	time.Sleep(time.Second * 5)
	if mng.workers[unhealthyCase].condition {
		t.Errorf("it should not be able to connect %s", unhealthyAddress)
	}
	if len(mng.workers) != 1 {
		t.Errorf("case: %s, Add worker failed", unhealthyCase)
	}
	mng.RemoveWorker(unhealthyCase)
	if len(mng.workers) != 0 {
		t.Errorf("case: %s, remove worker failed", unhealthyCase)
	}
}

var mng *Manager

func TestMain(m *testing.M) {
	mng = NewManager(context.TODO(), printCondition)
	code := m.Run()
	os.Exit(code)
}

func printCondition(uid string, isHealthy bool) error {
	fmt.Printf("health check result, uid: %s, isHealthy: %t\n", uid, isHealthy)
	return nil
}
