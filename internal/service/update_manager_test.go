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

func TestCalculateNextWallClockRun(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	baseH := 4
	baseM := 0

	// 1. Daily 24h interval
	tBefore := time.Date(2026, 10, 28, 1, 0, 0, 0, loc)
	next := CalculateNextWallClockRun(tBefore, 24.0, baseH, baseM)
	expected := time.Date(2026, 10, 28, 4, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Errorf("expected 24h next to be %v, got %v", expected, next)
	}

	tAfter := time.Date(2026, 10, 28, 5, 0, 0, 0, loc)
	nextAfter := CalculateNextWallClockRun(tAfter, 24.0, baseH, baseM)
	expectedAfter := time.Date(2026, 10, 29, 4, 0, 0, 0, loc)
	if !nextAfter.Equal(expectedAfter) {
		t.Errorf("expected 24h next after 5am to be %v, got %v", expectedAfter, nextAfter)
	}

	// 2. 12h interval
	next12_morning := CalculateNextWallClockRun(tBefore, 12.0, baseH, baseM)
	if !next12_morning.Equal(expected) {
		t.Errorf("expected 12h morning next to be %v, got %v", expected, next12_morning)
	}

	tAfterMorning := time.Date(2026, 10, 28, 4, 1, 0, 0, loc)
	next12_afternoon := CalculateNextWallClockRun(tAfterMorning, 12.0, baseH, baseM)
	expected12_afternoon := time.Date(2026, 10, 28, 16, 0, 0, 0, loc)
	if !next12_afternoon.Equal(expected12_afternoon) {
		t.Errorf("expected 12h afternoon next to be %v, got %v", expected12_afternoon, next12_afternoon)
	}

	tNight := time.Date(2026, 10, 28, 17, 0, 0, 0, loc)
	next12_night := CalculateNextWallClockRun(tNight, 12.0, baseH, baseM)
	if !next12_night.Equal(expectedAfter) {
		t.Errorf("expected 12h night next to be %v, got %v", expectedAfter, next12_night)
	}

	// 3. 6h interval
	t6 := time.Date(2026, 10, 28, 4, 1, 0, 0, loc)
	next6 := CalculateNextWallClockRun(t6, 6.0, baseH, baseM)
	expected6 := time.Date(2026, 10, 28, 10, 0, 0, 0, loc)
	if !next6.Equal(expected6) {
		t.Errorf("expected 6h next to be %v, got %v", expected6, next6)
	}

	// 4. Disabled
	next0 := CalculateNextWallClockRun(tBefore, 0, baseH, baseM)
	if !next0.IsZero() {
		t.Errorf("expected zero time for interval <= 0")
	}
}
