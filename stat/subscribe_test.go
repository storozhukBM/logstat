package stat

import (
	"github.com/storozhukBM/logstat/common/test"
	"sync/atomic"
	"testing"
	"time"
)

func TestSubscribe(t *testing.T) {
	t.Parallel()
	reporter := &reportProviderMock{reports: make(chan Report, 10)}
	count := uint64(0)
	testReport := Report{
		CycleDurationInSeconds:   12,
		CycleOffset:              1,
		CycleStartUnixTime:       10,
		TotalRequests:            123,
		TotalResponseSizeInBytes: 321,
		requestsPerSection:       map[string]uint64{"/a": 123},
		requestsPerStatusCode:    map[int32]uint64{200: 123},
	}
	_, subErr := NewReportSubscription(
		reporter,
		func(r Report) {
			test.Equals(t, testReport, r, "report mismatch in listener#1")
			atomic.AddUint64(&count, 1)
		},
		func(r Report) {
			test.Equals(t, testReport, r, "report mismatch in listener#2")
			atomic.AddUint64(&count, 1)
		},
		func(r Report) {
			test.Equals(t, testReport, r, "report mismatch in listener#3")
			atomic.AddUint64(&count, 1)
			panic("expected panic from test")
		},
	)
	test.FailOnError(t, subErr)

	reporter.reports <- testReport
	reporter.reports <- testReport
	time.Sleep(10 * defaultTimeout)
	test.Equals(t, uint64(6), atomic.LoadUint64(&count), "subscriptions should work even after panic")
}

type reportProviderMock struct {
	reports chan Report
}

func (l *reportProviderMock) Reports() <-chan Report {
	return l.reports
}
