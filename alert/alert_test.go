package alert

import (
	"fmt"
	"github.com/storozhukBM/logstat/common/test"
	"github.com/storozhukBM/logstat/stat"
	"testing"
	"time"
)

const defaultTimeout = 10 * time.Millisecond

func TestAlert(t *testing.T) {
	t.Parallel()
	state, stateErr := NewTrafficState(
		10, 2, 1, 2,
	)
	test.FailOnError(t, stateErr)
	state.Store(stat.Report{CycleDurationInSeconds: 2, CycleOffset: 3, CycleStartUnixTime: 6, TotalRequests: 4})
	waitForAlertTillTimeout(t, state)

	state.Store(stat.Report{CycleDurationInSeconds: 2, CycleOffset: 4, CycleStartUnixTime: 8, TotalRequests: 4})
	waitForAlertTillTimeout(t, state)

	state.Store(stat.Report{CycleDurationInSeconds: 2, CycleOffset: 5, CycleStartUnixTime: 10, TotalRequests: 2})
	waitForAlert(t, state, TrafficAlert{
		AlertID:                  1,
		Resolved:                 false,
		MaxAllowedRequests:       10,
		ObservedInWindowRequests: 10,
		WindowStartUnixTime:      0,
		WindowEndUnixTime:        10,
	})

	state.Store(stat.Report{CycleDurationInSeconds: 2, CycleOffset: 6, CycleStartUnixTime: 12, TotalRequests: 4})
	waitForAlert(t, state, TrafficAlert{
		AlertID:                  2,
		Resolved:                 false,
		MaxAllowedRequests:       10,
		ObservedInWindowRequests: 14,
		WindowStartUnixTime:      2,
		WindowEndUnixTime:        12,
	})

	state.Store(stat.Report{CycleDurationInSeconds: 2, CycleOffset: 11, CycleStartUnixTime: 22, TotalRequests: 3})
	waitForAlert(t, state, TrafficAlert{
		AlertID:                  2,
		Resolved:                 true,
		MaxAllowedRequests:       10,
		ObservedInWindowRequests: 7,
		WindowStartUnixTime:      12,
		WindowEndUnixTime:        22,
	})

	// test ring behavior of alerts channel
	state.Store(stat.Report{CycleDurationInSeconds: 2, CycleOffset: 16, CycleStartUnixTime: 32, TotalRequests: 30})
	state.Store(stat.Report{CycleDurationInSeconds: 2, CycleOffset: 17, CycleStartUnixTime: 34, TotalRequests: 40})
	state.Store(stat.Report{CycleDurationInSeconds: 2, CycleOffset: 18, CycleStartUnixTime: 36, TotalRequests: 50})

	waitForAlert(t, state, TrafficAlert{
		AlertID:                  4,
		Resolved:                 false,
		MaxAllowedRequests:       10,
		ObservedInWindowRequests: 70,
		WindowStartUnixTime:      24,
		WindowEndUnixTime:        34,
	})
	waitForAlert(t, state, TrafficAlert{
		AlertID:                  5,
		Resolved:                 false,
		MaxAllowedRequests:       10,
		ObservedInWindowRequests: 120,
		WindowStartUnixTime:      26,
		WindowEndUnixTime:        36,
	})
}

func waitForAlert(t *testing.T, state *TrafficState, expectedAlert TrafficAlert) {
	var timeout time.Time
	var alert TrafficAlert
	var open bool
	select {
	case alert, open = <-state.Alerts():
	case timeout = <-time.After(defaultTimeout):
	}
	test.Equals(t, time.Time{}, timeout, "no timeout should happen")
	test.Equals(t, expectedAlert, alert, "read alert mismatch")
	test.Equals(t, true, open, "ring shouldn't be closed")
}

func waitForAlertTillTimeout(t *testing.T, storage *TrafficState) {
	emptyTime := time.Time{}

	var timeout time.Time
	var alert TrafficAlert

	select {
	case alert, _ = <-storage.Alerts():
	case timeout = <-time.After(defaultTimeout):
	}
	test.Equals(t, TrafficAlert{}, alert, "read alert")
	if timeout == emptyTime {
		test.FailOnError(t, fmt.Errorf("timeout didn't happen"))
	}
}
