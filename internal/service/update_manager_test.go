package service

import (
	"context"
	"testing"
	"time"
)

func TestUpdateManager_StatusAndLifecycle(t *testing.T) {
	um := &UpdateManager{
		running: false,
	}

	// 1. Initial status
	if um.IsRunning() {
		t.Fatalf("expected running to be false initially")
	}

	status := um.GetStatusGraphQL()
	jobsInfo, ok := status["jobsInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected jobsInfo map in status")
	}
	if jobsInfo["isRunning"] != false {
		t.Fatalf("expected isRunning to be false in jobsInfo")
	}
	if jobsInfo["totalJobs"] != 0 || jobsInfo["finishedJobs"] != 0 {
		t.Fatalf("expected 0 total and finished jobs")
	}

	// 2. Set interval and scheduler testing
	um.SetInterval(24.0)
	if um.currentInterval != 24.0 {
		t.Fatalf("expected currentInterval 24.0, got %g", um.currentInterval)
	}

	// Cancel/Stop scheduler
	um.SetInterval(0)
	if um.schedulerStop != nil {
		t.Fatalf("expected schedulerStop to be nil when interval is 0")
	}

	// 3. Test cancel when not running
	um.CancelUpdate()
	if um.IsRunning() {
		t.Fatalf("expected running to be false after cancel")
	}
}

func TestUpdateManager_Helpers(t *testing.T) {
	if !parseBoolSetting("true", false) || !parseBoolSetting("1", false) {
		t.Errorf("parseBoolSetting failed for true/1")
	}
	if parseBoolSetting("false", true) || parseBoolSetting("0", true) {
		t.Errorf("parseBoolSetting failed for false/0")
	}
	if !parseBoolSetting("", true) {
		t.Errorf("parseBoolSetting failed for empty fallback true")
	}

	if parseIntSetting("42", 0) != 42 {
		t.Errorf("parseIntSetting failed for 42")
	}
	if parseIntSetting("", 10) != 10 {
		t.Errorf("parseIntSetting failed for empty fallback 10")
	}
}

func TestUpdateManager_CancelActive(t *testing.T) {
	um := &UpdateManager{}
	um.running = true
	ctx, cancel := context.WithCancel(context.Background())
	um.cancelFunc = cancel

	um.CancelUpdate()

	if um.IsRunning() {
		t.Errorf("expected running to be false after CancelUpdate")
	}

	select {
	case <-ctx.Done():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Errorf("expected context to be cancelled")
	}
}
