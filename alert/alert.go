package alert

import (
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/stat"
)

/*
A component used to accept traffic stats reports and examine them for certain violations.

Responsibilities:
	- accept traffic stats reports
	- maintain an internal ring of reports that fit into specified alerting period
	- emmit events about new or resolved alerts into the output channel

Attention:
	- `Store` method is not safe for concurrent use and intent to use in
	combination with `stat.ReportSubscription` component or synchronized externally
	- if alerts from output channel won't be consumed this component will print them as
	error report
*/
type TrafficState struct {
	windowDurationInSeconds int64
	reportsCycleInSeconds   int64
	maxTrafficInWindow      uint64

	requestsInWindow             uint64
	lastReportCycleStartUnixTime int64
	reportsRing                  *trafficEvictingQueue

	alertsCount uint64
	current     *TrafficAlert
	alertsRing  chan TrafficAlert
}

func NewTrafficState(
	windowDurationInSeconds uint64, reportsCycleInSeconds uint64,
	maxAvgTrafficInReqPerSecond uint64, alertRingSize uint,
) (*TrafficState, error) {
	slotsSize := windowDurationInSeconds / reportsCycleInSeconds
	if slotsSize < 1 {
		return nil, fmt.Errorf("mismatched configuration of windowDurationInSeconds and reportsCycleInSeconds")
	}
	if slotsSize > 4096 {
		return nil, fmt.Errorf("windowDurationInSeconds is too big for such report cycle size")
	}
	if alertRingSize < 1 {
		return nil, fmt.Errorf("alertRingSize should be at least 1")
	}
	result := &TrafficState{
		windowDurationInSeconds: int64(windowDurationInSeconds),
		reportsCycleInSeconds:   int64(reportsCycleInSeconds),
		maxTrafficInWindow:      maxAvgTrafficInReqPerSecond * windowDurationInSeconds,

		requestsInWindow:             0,
		lastReportCycleStartUnixTime: 0,
		reportsRing:                  newTrafficRing(int(slotsSize)),

		alertsRing: make(chan TrafficAlert, alertRingSize),
	}
	return result, nil
}

func (s *TrafficState) Alerts() <-chan TrafficAlert {
	return s.alertsRing
}

func (s *TrafficState) Store(report stat.Report) {
	if s.reportsCycleInSeconds != report.CycleDurationInSeconds {
		log.Error(
			"expected cycle size: %v; actual cycle size: %v",
			s.reportsCycleInSeconds, report.CycleDurationInSeconds,
		)
		return
	}

	windowStartUnixTime := report.CycleStartUnixTime - s.windowDurationInSeconds
	for {
		head, ok := s.reportsRing.getHead()
		if !ok || head.cycleStartUnixTime >= windowStartUnixTime {
			break
		}
		s.requestsInWindow -= head.cycleRequests
		s.reportsRing.removeHead()
	}

	s.reportsRing.pushToTail(trafficSlot{cycleRequests: report.TotalRequests, cycleStartUnixTime: report.CycleStartUnixTime})
	s.requestsInWindow += report.TotalRequests
	s.lastReportCycleStartUnixTime = report.CycleStartUnixTime
	s.checkForAlertsViolation(report)
}

func (s *TrafficState) checkForAlertsViolation(report stat.Report) {
	if s.requestsInWindow >= s.maxTrafficInWindow {
		s.alertsCount++
		s.current = &TrafficAlert{
			AlertID:                  s.alertsCount,
			Resolved:                 false,
			MaxAllowedRequests:       s.maxTrafficInWindow,
			ObservedInWindowRequests: s.requestsInWindow,
			WindowStartUnixTime:      report.CycleStartUnixTime - s.windowDurationInSeconds,
			WindowEndUnixTime:        report.CycleStartUnixTime,
		}
		s.pushAlertToRing(*s.current)
		return
	}
	if s.current == nil {
		return
	}
	s.pushAlertToRing(TrafficAlert{
		AlertID:                  s.current.AlertID,
		Resolved:                 true,
		MaxAllowedRequests:       s.maxTrafficInWindow,
		ObservedInWindowRequests: s.requestsInWindow,
		WindowStartUnixTime:      report.CycleStartUnixTime - s.windowDurationInSeconds,
		WindowEndUnixTime:        report.CycleStartUnixTime,
	})
	s.current = nil
}

func (s *TrafficState) pushAlertToRing(a TrafficAlert) {
	select {
	case s.alertsRing <- a:
	default:
		oldAlert := <-s.alertsRing
		log.Error("[ALERT] TrafficAlert wasn't consumed from TrafficStateAlert: %+v", oldAlert)
		s.alertsRing <- a
	}
}

type trafficSlot struct {
	cycleRequests      uint64
	cycleStartUnixTime int64
}

type trafficEvictingQueue struct {
	ring []trafficSlot
	head int
	tail int
	size int
}

func newTrafficRing(size int) *trafficEvictingQueue {
	return &trafficEvictingQueue{ring: make([]trafficSlot, size)}
}

func (q *trafficEvictingQueue) getHead() (trafficSlot, bool) {
	if q.size == 0 {
		return trafficSlot{}, false
	}
	head := q.ring[q.head]
	return head, true
}

func (q *trafficEvictingQueue) removeHead() (trafficSlot, bool) {
	if q.size == 0 {
		return trafficSlot{}, false
	}
	head := q.ring[q.head]
	q.moveHead()
	q.size--
	return head, true
}

func (q *trafficEvictingQueue) pushToTail(slot trafficSlot) {
	if q.size == len(q.ring) {
		// required only for situations when server time in report is invalid or going backwards
		// this change helps alert manager to handle such situations gracefully
		q.removeHead()
	}
	q.ring[q.tail] = slot
	q.moveTail()
	q.size++
}

func (q *trafficEvictingQueue) moveHead() {
	q.head = (q.head + 1) % len(q.ring)
}

func (q *trafficEvictingQueue) moveTail() {
	q.tail = (q.tail + 1) % len(q.ring)
}
