package main

import (
	"os/exec"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// waitFor — kosul saglanana kadar kisa araliklarla poll eder, timeout'ta test'i basarisiz yapar.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("kosul %s icinde saglanmadi", timeout)
}

func newSleepSupervisor() *Supervisor {
	return &Supervisor{
		Spawn:   func() *exec.Cmd { return exec.Command("sleep", "30") },
		Backoff: 100 * time.Millisecond,
	}
}

func TestSupervisorStartSpawns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test (sleep)")
	}
	s := newSleepSupervisor()
	s.Start()
	waitFor(t, 2*time.Second, func() bool { return s.currentPID() != 0 })
	s.Shutdown()
}

func TestSupervisorRestartChangesPID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test (sleep)")
	}
	s := newSleepSupervisor()
	s.Start()
	waitFor(t, 2*time.Second, func() bool { return s.currentPID() != 0 })
	firstPID := s.currentPID()

	s.Restart()
	waitFor(t, 2*time.Second, func() bool {
		pid := s.currentPID()
		return pid != 0 && pid != firstPID
	})
	s.Shutdown()
}

func TestSupervisorShutdownStopsRespawn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test (sleep)")
	}
	s := newSleepSupervisor()
	s.Start()
	waitFor(t, 2*time.Second, func() bool { return s.currentPID() != 0 })

	s.Shutdown()
	// Shutdown sonrasi kisa bir sure bekle: respawn OLMAMALI.
	time.Sleep(500 * time.Millisecond)
	if pid := s.currentPID(); pid != 0 {
		// cmd hala eski (oldurulmus) surecin referansi olabilir; asil kontrol PID'in
		// gercekten olup olmadigi (kill -0 ile).
		if err := exec.Command("kill", "-0", strconv.Itoa(pid)).Run(); err == nil {
			t.Fatalf("shutdown sonrasi surec hala calisiyor gibi gorunuyor (pid=%d)", pid)
		}
	}
}
