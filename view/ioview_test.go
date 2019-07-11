package view

import (
	"bytes"
	"context"
	"github.com/storozhukBM/logstat/alert"
	"github.com/storozhukBM/logstat/common/test"
	"github.com/storozhukBM/logstat/stat"
	"testing"
	"time"
)

const defaultTimeout = 100 * time.Millisecond

const expectedReport = `
|
| Report Summary
|_________________________________ _________________________________
| Server Time                       1970-01-01 00:05:00 +0000 UTC
|_________________________________ _________________________________
| Total Requests                                              123
|_________________________________ _________________________________
| Total Responses Size [KBs]                              40.2676
|_________________________________ _________________________________
| Average Requests Rate [req/sec]                         12.3000
|_________________________________ _________________________________
| Response Throughput [KBs/sec]                            4.0268
|_________________________________ _________________________________
| Average Response Size [KBs/req]                          0.3274
|_________________________________ _________________________________

|
| Section TOP
|_________________________________ _________________________________
| Section                           Requests
|_________________________________ _________________________________
| /user                                                        32
|_________________________________ _________________________________
| /api                                                         10
|_________________________________ _________________________________
| /report                                                       3
|_________________________________ _________________________________

|
| Status Code TOP
|_________________________________ _________________________________
| Status Code                       Requests
|_________________________________ _________________________________
| 201                                                          20
|_________________________________ _________________________________
| 200                                                          15
|_________________________________ _________________________________
| 400                                                          10
|_________________________________ _________________________________

`

func TestIOReport(t *testing.T) {
	t.Parallel()
	reportBuf := bytes.NewBuffer(nil)
	v, vErr := NewIOView(context.Background(), 10*time.Second, reportBuf)
	test.FailOnError(t, vErr)

	report := stat.BuildReport(
		map[string]uint64{"/report": 3, "/api": 10, "/user": 32},
		map[int32]uint64{200: 15, 201: 20, 400: 10},
	)
	report.CycleDurationInSeconds = 10
	report.CycleOffset = 30
	report.CycleStartUnixTime = 300
	report.TotalRequests = 123
	report.TotalResponseSizeInBytes = 41234

	v.Report(report)
	time.Sleep(defaultTimeout)
	test.Equals(t, expectedReport, string(reportBuf.Bytes()), "report")
}

const expAlert = "[ALERT] Time: 1970-01-01 00:02:00 +0000 UTC; Max Average Requests Rate [req/sec]: 1.2500; Observed Average Requests Rate: 2.5000\n"
const expResolved = "[RESOLVED] Time: 1970-01-01 00:02:10 +0000 UTC; Max Average Requests Rate [req/sec]: 1.2500; Observed Average Requests Rate: 1.2417\n"

func TestIOAlert(t *testing.T) {
	t.Parallel()
	buf := bytes.NewBuffer(nil)
	v, vErr := NewIOView(context.Background(), 10*time.Second, buf)
	test.FailOnError(t, vErr)
	{
		v.TrafficAlert(alert.TrafficAlert{
			AlertID:                  1,
			Resolved:                 false,
			MaxAllowedRequests:       150,
			ObservedInWindowRequests: 300,
			WindowStartUnixTime:      0,
			WindowEndUnixTime:        120,
		})
		time.Sleep(defaultTimeout)
		test.Equals(t, []byte(expAlert), buf.Bytes(), "alert mismatch")
	}

	buf.Reset()
	{
		v.TrafficAlert(alert.TrafficAlert{
			AlertID:                  1,
			Resolved:                 true,
			MaxAllowedRequests:       150,
			ObservedInWindowRequests: 149,
			WindowStartUnixTime:      10,
			WindowEndUnixTime:        130,
		})
		time.Sleep(defaultTimeout)
		test.Equals(t, []byte(expResolved), buf.Bytes(), "resolved mismatch")
	}
}
