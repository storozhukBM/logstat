package view

import (
	"context"
	"github.com/storozhukBM/logstat/common/test"
	"github.com/storozhukBM/logstat/stat"
	"os"
	"testing"
	"time"
)

func TestIOView(t *testing.T) {
	v, vErr := NewIOView(context.Background(), 10*time.Second, os.Stdout)
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
	time.Sleep(10 * time.Millisecond)
}
