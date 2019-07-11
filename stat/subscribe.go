package stat

import (
	"fmt"
	"github.com/storozhukBM/logstat/common/pnc"
)

type reportProvider interface {
	Reports() <-chan Report
}

/*
A component used broadcast reports to multiple consumers.
*/
type ReportSubscription struct {
	reportProvider reportProvider
	listeners      []func(r Report)
}

func NewReportSubscription(reportProvider reportProvider, listeners ...func(r Report)) (*ReportSubscription, error) {
	if reportProvider == nil {
		return nil, fmt.Errorf("reportProvider can't be nil")
	}
	if reportProvider.Reports() == nil {
		return nil, fmt.Errorf("reportProvider reports chan can't be nil")
	}
	result := &ReportSubscription{reportProvider: reportProvider, listeners: listeners}
	go result.run()
	return result, nil
}

func (s *ReportSubscription) run() {
	if s.listeners == nil {
		return
	}
	for report := range s.reportProvider.Reports() {
		for _, listener := range s.listeners {
			if listener == nil {
				continue
			}
			func() {
				defer pnc.PanicHandle()
				listener(report)
			}()
		}
	}
}
