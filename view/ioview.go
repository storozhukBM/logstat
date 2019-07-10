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

const sep = "_______________________________"

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
	_, _ = fmt.Fprintf(v.output, "\n|\n| Report Summary\n")
	w := v.newTable()
	v.printRowToTable(w, "|%s\t%s\t%s\n", sep, sep, sep)

	v.printRowToTable(w, "| Server Time\t Total requests\t Total response size\n")
	v.printRowToTable(w, "|%s\t%s\t%s\n", sep, sep, sep)

	v.printRowToTable(
		w, "| %+v\t %+v\t %+v\n",
		time.Unix(r.CycleStartUnixTime, 0).UTC(), r.TotalRequests, r.TotalResponseSizeInBytes,
	)
	v.printRowToTable(w, "|%s\t%s\t%s\n", sep, sep, sep)
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
		v.printRowToTable(w, "| %v\t %v\n", sectionHit.section, sectionHit.hits)
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
		v.printRowToTable(w, "| %v\t %v\n", codeHit.code, codeHit.hits)
		v.printRowToTable(w, "|%s\t%s\n", sep, sep)
	}
	v.finishTable(w)
}

func (v *IOView) printTrafficAlert(a alert.TrafficAlert) {
	if !a.Resolved {
		v.lastTrafficAlert = &a
		_, _ = fmt.Fprintf(v.output, "[ALERT]: %+v\n", a)
		return
	}
	_, _ = fmt.Fprintf(v.output, "[RESOLVED]: %+v\n", a)
	v.lastTrafficAlert = nil
}

func (v *IOView) printNoTrafficReport() {
	_, _ = fmt.Fprint(v.output, "| Report Summary: no traffic\n")
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
