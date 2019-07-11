package alert

import (
	"github.com/storozhukBM/logstat/common/test"
	"sync/atomic"
	"testing"
	"time"
)

func TestSubscribe(t *testing.T) {
	t.Parallel()
	alerter := &alertProviderMock{alerts: make(chan TrafficAlert, 10)}
	count := uint64(0)
	testAlert := TrafficAlert{
		AlertID:                  2,
		Resolved:                 false,
		MaxAllowedRequests:       12,
		ObservedInWindowRequests: 13,
		WindowStartUnixTime:      10,
		WindowEndUnixTime:        20,
	}
	_, subErr := NewAlertsSubscription(
		alerter,
		func(a TrafficAlert) {
			test.Equals(t, testAlert, a, "alert mismatch in listener#1")
			atomic.AddUint64(&count, 1)
		},
		func(a TrafficAlert) {
			test.Equals(t, testAlert, a, "alert mismatch in listener#2")
			atomic.AddUint64(&count, 1)
		},
		func(a TrafficAlert) {
			test.Equals(t, testAlert, a, "alert mismatch in listener#3")
			atomic.AddUint64(&count, 1)
			panic("expected panic from test")
		},
	)
	test.FailOnError(t, subErr)

	alerter.alerts <- testAlert
	alerter.alerts <- testAlert
	time.Sleep(10 * defaultTimeout)
	test.Equals(t, uint64(6), atomic.LoadUint64(&count), "subscriptions should work even after panic")
}

type alertProviderMock struct {
	alerts chan TrafficAlert
}

func (l *alertProviderMock) Alerts() <-chan TrafficAlert {
	return l.alerts
}
