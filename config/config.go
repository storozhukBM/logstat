package config

import (
	"flag"
	"time"
)

type Config struct {
	DebugMode bool

	FileName               string
	FileReadBufSizeInBytes uint
	FileReadPollPeriod     time.Duration

	W3CParserSectionsStringCacheSize uint

	TrafficStatAggregationPeriodInSeconds uint64
	TrafficStatAggregationCyclesRingSize  uint

	TrafficAlertAggregationPeriodInSeconds uint64
	TrafficAlertMaxTrafficInReqPerSecond   uint64
	TrafficAlertAggregationRingSize        uint

	IOViewRefreshPeriod time.Duration
}

func ParseFlagsAsConfig() Config {
	c := Config{}

	flag.BoolVar(&c.DebugMode, "debugMode", false, "enable debug mode")

	flag.StringVar(&c.FileName, "fileName", "/tmp/access.log", "log file to monitor")
	flag.UintVar(
		&c.FileReadBufSizeInBytes, "fileReadBufSizeInBytes", 16*1024,
		"size of buffer used to read lines from log",
	)
	flag.DurationVar(
		&c.FileReadPollPeriod, "fileReadPollPeriod", 100*time.Millisecond,
		"period of file poll in case when there is no new lines to read",
	)

	flag.UintVar(
		&c.W3CParserSectionsStringCacheSize, "w3cParserSectionsStringCacheSize", 16*1024,
		"size of cache that eliminates allocation of parsed `sections`. Make it bigger than estimated count of sections",
	)

	flag.Uint64Var(
		&c.TrafficStatAggregationPeriodInSeconds, "trafficStatAggregationPeriodInSeconds", 10,
		"window size in seconds for traffic report aggregation",
	)
	flag.UintVar(
		&c.TrafficStatAggregationCyclesRingSize, "trafficStatAggregationCyclesRingSize", 10,
		"size of ring buffer with aggregated traffic reports",
	)

	flag.Uint64Var(
		&c.TrafficAlertAggregationPeriodInSeconds, "trafficAlertAggregationPeriodInSeconds", 120,
		"window size for alert traffic aggregation",
	)
	flag.Uint64Var(
		&c.TrafficAlertMaxTrafficInReqPerSecond, "trafficAlertMaxTrafficInReqPerSecond", 10,
		"limit of throughput that should trigger alert",
	)
	flag.UintVar(
		&c.TrafficAlertAggregationRingSize, "trafficAlertAggregationRingSize", 10,
		"size of ring buffer with aggregated alerts",
	)

	flag.DurationVar(
		&c.IOViewRefreshPeriod, "ioViewRefreshPeriod", 10*time.Second,
		"period of time to emit heart beat into view",
	)

	flag.Parse()
	return c
}
