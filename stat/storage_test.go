package stat

import (
	"fmt"
	"github.com/storozhukBM/logstat/common/test"
	"testing"
	"time"
)

const defaultTimeout = 10 * time.Millisecond

func TestStatsStorage(t *testing.T) {
	t.Parallel()
	storage, storageErr := NewStorage(10, 2)
	test.FailOnError(t, storageErr)

	storage.Store(Record{UnixTime: 1, Section: "first", StatusCode: 200, ResponseSize: 5})
	waitForCycleTillTimeout(t, storage)

	{
		storage.Store(Record{UnixTime: 11, Section: "first", StatusCode: 200, ResponseSize: 7})
		waitForReport(t, storage, Report{
			CycleDurationInSeconds:   10,
			CycleOffset:              0,
			CycleStartUnixTime:       0,
			TotalRequests:            1,
			TotalResponseSizeInBytes: 5,
			requestsPerSection:       map[string]uint64{"first": 1},
			requestsPerStatusCode:    map[int32]uint64{200: 1},
		})

		storage.Store(Record{UnixTime: 11, Section: "first", StatusCode: 500, ResponseSize: 3})
		waitForCycleTillTimeout(t, storage)
		storage.Store(Record{UnixTime: 15, Section: "second", StatusCode: 200, ResponseSize: 3})
		waitForCycleTillTimeout(t, storage)
		storage.Store(Record{UnixTime: 19, Section: "first", StatusCode: 500, ResponseSize: 20})
		waitForCycleTillTimeout(t, storage)
	}

	storage.Store(Record{UnixTime: 20, Section: "first", StatusCode: 200, ResponseSize: 7})
	waitForReport(t, storage, Report{
		CycleDurationInSeconds:   10,
		CycleOffset:              1,
		CycleStartUnixTime:       10,
		TotalRequests:            4,
		TotalResponseSizeInBytes: 33,
		requestsPerSection:       map[string]uint64{"first": 3, "second": 1},
		requestsPerStatusCode:    map[int32]uint64{200: 2, 500: 2},
	})

	storage.Store(Record{UnixTime: 30, Section: "third", StatusCode: 200, ResponseSize: 7})
	storage.Store(Record{UnixTime: 46, Section: "other", StatusCode: 400, ResponseSize: 7})
	storage.Store(Record{UnixTime: 52, Section: "some", StatusCode: 201, ResponseSize: 7})

	waitForReport(t, storage, Report{
		CycleDurationInSeconds:   10,
		CycleOffset:              3,
		CycleStartUnixTime:       30,
		TotalRequests:            1,
		TotalResponseSizeInBytes: 7,
		requestsPerSection:       map[string]uint64{"third": 1},
		requestsPerStatusCode:    map[int32]uint64{200: 1},
	})
	waitForReport(t, storage, Report{
		CycleDurationInSeconds:   10,
		CycleOffset:              4,
		CycleStartUnixTime:       40,
		TotalRequests:            1,
		TotalResponseSizeInBytes: 7,
		requestsPerSection:       map[string]uint64{"other": 1},
		requestsPerStatusCode:    map[int32]uint64{400: 1},
	})
}

func waitForReport(t *testing.T, storage *Storage, expectedReport Report) {
	var timeout time.Time
	var report Report
	var open bool
	select {
	case report, open = <-storage.Reports():
	case timeout = <-time.After(defaultTimeout):
	}
	test.Equals(t, time.Time{}, timeout, "no timeout should happen")
	test.Equals(t, expectedReport, report, "read report mismatch")
	test.Equals(t, true, open, "ring shouldn't be closed")
}

func waitForCycleTillTimeout(t *testing.T, storage *Storage) {
	emptyTime := time.Time{}

	var timeout time.Time
	var report Report

	select {
	case report, _ = <-storage.Reports():
	case timeout = <-time.After(defaultTimeout):
	}
	test.Equals(t, Report{}, report, "read report")
	if timeout == emptyTime {
		test.FailOnError(t, fmt.Errorf("timeout didn't happen"))
	}
}
