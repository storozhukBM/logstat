package view

import (
	"context"
	"errors"
	"fmt"
	"github.com/storozhukBM/logstat/alert"
	"github.com/storozhukBM/logstat/common/pnc"
	"github.com/storozhukBM/logstat/stat"
	"io"
	"sort"
	"text/tabwriter"
	"time"
)

const sep = "_________________________________"

/*
A component used to visualize reports and print alerts to the specified writer.

Responsibilities:
	- print stats reports
	- print alerts
	- print heartbeats in case of no other events

Attention:
	- this component allocated a lot, but it shouldn't be a problem
	due to the expected low rate of events

Future:
	- current reports are human-readable, but can be harder to parse by machine,
	print templates can be abstracted and replaceable to allow different print formats.

*/
type IOView struct {
	ctx           context.Context
	refreshPeriod time.Duration
	output        io.Writer

	lastTrafficAlert *alert.TrafficAlert
	alerts           chan alert.TrafficAlert
	reports          chan stat.Report
}

func NewIOView(ctx context.Context, refreshPeriod time.Duration, output io.Writer) (*IOView, error) {
	if output == nil {
		return nil, errors.New("output can't be nil")
	}
	result := &IOView{
		ctx:           ctx,
		refreshPeriod: refreshPeriod,
		output:        output,

		lastTrafficAlert: nil,
		alerts:           make(chan alert.TrafficAlert, 8),
		reports:          make(chan stat.Report, 8),
	}
	go result.run()
	return result, nil
}

func (v *IOView) TrafficAlert(a alert.TrafficAlert) {
	v.alerts <- a
}

func (v *IOView) Report(r stat.Report) {
	v.reports <- r
}

func (v *IOView) run() {
	cycle := func() {
		defer pnc.PanicHandle()
		select {
		case a := <-v.alerts:
			v.printTrafficAlert(a)
		case r := <-v.reports:
			v.printReport(r)
		case <-time.After(v.refreshPeriod):
			v.printNoTrafficReport()
		case <-v.ctx.Done():
			return
		}
	}
	for v.ctx.Err() == nil {
		cycle()
	}
}

func (v *IOView) printReport(r stat.Report) {
	v.printReportSummary(r)
	v.printSectionTop(r)
	v.printStatusCodeTop(r)
}

func (v *IOView) printReportSummary(r stat.Report) {
	reqPerSecond := float64(r.TotalRequests) / float64(r.CycleDurationInSeconds)
	KBPerSecond := float64(r.TotalResponseSizeInBytes) / float64(r.CycleDurationInSeconds) / 1024.
	KBPerRequest := float64(r.TotalResponseSizeInBytes) / float64(r.TotalRequests) / 1024.
	totalKBs := float64(r.TotalResponseSizeInBytes) / 1024.

	_, _ = fmt.Fprintf(v.output, "\n|\n| Report Summary\n")
	w := v.newTable()
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	v.printRowToTable(w, "| Server Time\t %v\n", time.Unix(r.CycleStartUnixTime, 0).UTC())
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	v.printRowToTable(w, "| Total Requests\t %29d\n", r.TotalRequests)
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	v.printRowToTable(w, "| Total Responses Size [KBs]\t %29.4f\n", totalKBs)
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	v.printRowToTable(w, "| Average Requests Rate [req/sec]\t %29.4f\n", reqPerSecond)
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	v.printRowToTable(w, "| Response Throughput [KBs/sec]\t %29.4f\n", KBPerSecond)
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	v.printRowToTable(w, "| Average Response Size [KBs/req]\t %29.4f\n", KBPerRequest)
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	v.finishTable(w)
}

func (v *IOView) printSectionTop(r stat.Report) {
	type sectionHit struct {
		section string
		hits    uint64
	}
	var sectionHits []sectionHit
	r.IterRequestsPerSection(func(section string, requests uint64) {
		sectionHits = append(sectionHits, sectionHit{section: section, hits: requests})
	})
	if sectionHits == nil {
		return
	}
	sort.Slice(sectionHits, func(i, j int) bool {
		return sectionHits[i].hits > sectionHits[j].hits
	})

	_, _ = fmt.Fprintf(v.output, "|\n| Section TOP\n")
	w := v.newTable()
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	v.printRowToTable(w, "| Section\t Requests\n")
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	for _, sectionHit := range sectionHits {
		v.printRowToTable(w, "| %v\t %29d\n", sectionHit.section, sectionHit.hits)
		v.printRowToTable(w, "|%s\t%s\n", sep, sep)
	}
	v.finishTable(w)
}

func (v *IOView) printStatusCodeTop(r stat.Report) {
	type statusCodeHit struct {
		code int32
		hits uint64
	}
	var codeHits []statusCodeHit
	r.IterRequestsPerStatusCode(func(code int32, requests uint64) {
		codeHits = append(codeHits, statusCodeHit{code: code, hits: requests})
	})
	if codeHits == nil {
		return
	}
	sort.Slice(codeHits, func(i, j int) bool {
		return codeHits[i].hits > codeHits[j].hits
	})

	_, _ = fmt.Fprintf(v.output, "|\n| Status Code TOP\n")
	w := v.newTable()

	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	v.printRowToTable(w, "| Status Code\t Requests\n")
	v.printRowToTable(w, "|%s\t%s\n", sep, sep)

	for _, codeHit := range codeHits {
		v.printRowToTable(w, "| %v\t %29d\n", codeHit.code, codeHit.hits)
		v.printRowToTable(w, "|%s\t%s\n", sep, sep)
	}
	v.finishTable(w)
}

func (v *IOView) printTrafficAlert(a alert.TrafficAlert) {
	windowDurationInSeconds := a.WindowEndUnixTime - a.WindowStartUnixTime
	observedReqPerSecond := float64(a.ObservedInWindowRequests) / float64(windowDurationInSeconds)
	maxAllowedReqPerSecond := float64(a.MaxAllowedRequests) / float64(windowDurationInSeconds)
	if a.Resolved {
		_, _ = fmt.Fprintf(v.output, "[RESOLVED] ")
		v.lastTrafficAlert = nil
	} else {
		_, _ = fmt.Fprintf(v.output, "[ALERT] ")
		v.lastTrafficAlert = &a
	}
	_, _ = fmt.Fprintf(
		v.output, "Time: %+v; Max Average Requests Rate [req/sec]: %.4f; Observed Average Requests Rate: %.4f\n",
		time.Unix(a.WindowEndUnixTime, 0).UTC(), maxAllowedReqPerSecond, observedReqPerSecond,
	)
}

func (v *IOView) printNoTrafficReport() {
	_, _ = fmt.Fprint(v.output, "| Report Summary: no traffic\n")
	if v.lastTrafficAlert != nil {
		windowSeconds := v.lastTrafficAlert.WindowEndUnixTime - v.lastTrafficAlert.WindowStartUnixTime
		secondsToNow := time.Now().Unix() - v.lastTrafficAlert.WindowEndUnixTime
		if secondsToNow > windowSeconds {
			_, _ = fmt.Fprint(v.output, "[WARN] alert may be resolved but there is no traffic to confirm\n")
		}
	}
}

func (v *IOView) newTable() *tabwriter.Writer {
	w := tabwriter.NewWriter(
		v.output, 10, 0, 1, ' ',
		tabwriter.TabIndent,
	)
	return w
}

func (v *IOView) printRowToTable(w *tabwriter.Writer, format string, args ...interface{}) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func (v *IOView) finishTable(w *tabwriter.Writer) {
	_ = w.Flush()
	_, _ = fmt.Fprintf(v.output, "\n")
}
