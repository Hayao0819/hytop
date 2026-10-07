package proc

import "testing"

func TestCPURateUsesTheLatestIntervalAndRejectsPIDReuse(t *testing.T) {
	t.Parallel()

	if got := cpuRate(10, 11.5, 0.5, true); got != 300 {
		t.Errorf("interval rate = %v, want 300", got)
	}
	if got := cpuRate(10, 11.5, 0.5, false); got != 0 {
		t.Errorf("reused PID rate = %v, want 0", got)
	}
	if got := cpuRate(10, 9, 1, true); got != 0 {
		t.Errorf("decreasing counter rate = %v, want 0", got)
	}
}
