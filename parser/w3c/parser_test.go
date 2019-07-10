package w3c

import (
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/common/test"
	"github.com/storozhukBM/logstat/stat"
	"testing"
)

type parsingCase struct {
	line   string
	result stat.Record
}

func TestW3CParsing(t *testing.T) {
	log.GlobalDebugEnabled = true
	parserWithCache, parserWithCacheErr := NewLineToStoreRecordParser(10)
	test.FailOnError(t, parserWithCacheErr)
	parserNoCache, parserNoCacheErr := NewLineToStoreRecordParser(0)
	test.FailOnError(t, parserNoCacheErr)

	cases := []parsingCase{
		{
			line:   `127.0.0.1 - james [09/May/2018:16:00:39 +0000] "GET /report HTTP/1.0" 200 123`,
			result: stat.Record{UnixTime: 1525881639, Section: "/report", StatusCode: 200, ResponseSize: 123},
		},
		{
			line:   `127.0.0.1 - jill [09/May/2018:16:00:41 +0000] "GET /api/user HTTP/1.0" 200 234`,
			result: stat.Record{UnixTime: 1525881641, Section: "/api", StatusCode: 200, ResponseSize: 234},
		},
		{
			line:   `127.0.0.1 - frank [09/May/2018:16:00:42 +0000] "POST /api/user HTTP/1.0" 200 34`,
			result: stat.Record{UnixTime: 1525881642, Section: "/api", StatusCode: 200, ResponseSize: 34},
		},
		{
			line:   `127.0.0.1 - mary [09/May/2018:16:00:42 +0000] "POST /api/user HTTP/1.0" 503 12`,
			result: stat.Record{UnixTime: 1525881642, Section: "/api", StatusCode: 503, ResponseSize: 12},
		},
	}

	parsers := []*LineToStoreRecordParser{parserWithCache, parserNoCache}

	for _, parser := range parsers {
		for _, testCase := range cases {
			t.Run(fmt.Sprintf("cacheSize_%v_case_", parser.sectionInternCacheSize), func(t *testing.T) {
				actual, err := parser.Parse([]byte(testCase.line))
				test.FailOnError(t, err)
				test.Equals(t, testCase.result, actual, "mismatch on: %s", testCase.line)
			})
		}
	}
}
