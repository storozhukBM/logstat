package alert

import (
	"fmt"
	"github.com/storozhukBM/logstat/common/pnc"
)

type alertsProvider interface {
	Alerts() <-chan TrafficAlert
}

type AlertsSubscription struct {
	reportProvider alertsProvider
	listeners      []func(a TrafficAlert)
}

func NewAlertsSubscription(reportProvider alertsProvider, listeners ...func(a TrafficAlert)) (*AlertsSubscription, error) {
	if reportProvider == nil {
		return nil, fmt.Errorf("alertsProvider can't be nil")
	}
	if reportProvider.Alerts() == nil {
		return nil, fmt.Errorf("alertsProvider alerts chan can't be nil")
	}
	result := &AlertsSubscription{reportProvider: reportProvider, listeners: listeners}
	go result.run()
	return result, nil
}

func (s *AlertsSubscription) run() {
	if s.listeners == nil {
		return
	}
	for report := range s.reportProvider.Alerts() {
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
