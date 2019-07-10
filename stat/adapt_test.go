package stat

import (
	"errors"
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/common/test"
	"strings"
	"testing"
	"time"
)

func TestNewLogToStoreAdapter(t *testing.T) {
	t.Parallel()
	logP := &logProviderMock{lines: make(chan []byte, 10)}
	storage := &storeMock{records: make(chan Record, 10)}
	parser := &parserMock{}
	_, adapterErr := NewLogToStoreAdapter(logP, storage, parser)
	test.FailOnError(t, adapterErr)

	logP.lines <- []byte("first")
	waitForRecord(t, storage, "first")
	waitForRecordTimeout(t, storage)

	logP.lines <- []byte("err: can't parse")
	waitForRecordTimeout(t, storage)

	logP.lines <- []byte("second")
	logP.lines <- []byte("third")
	waitForRecord(t, storage, "second")
	waitForRecord(t, storage, "third")
	waitForRecordTimeout(t, storage)

	logP.lines <- []byte("pnc: expected panic for tests")
	waitForRecordTimeout(t, storage)

	logP.lines <- []byte("fourth")
	waitForRecord(t, storage, "fourth")
}

func waitForRecord(t *testing.T, storage *storeMock, expSection string) {
	{
		var timeout time.Time
		var record Record
		var open bool
		select {
		case record, open = <-storage.records:
		case timeout = <-time.After(defaultTimeout):
		}
		test.Equals(t, time.Time{}, timeout, "no timeout should happen")
		test.Equals(t, expSection, record.Section, "read record mismatch")
		test.Equals(t, true, open, "ring shouldn't be closed")
	}
}

func waitForRecordTimeout(t *testing.T, storage *storeMock) {
	emptyTime := time.Time{}

	var timeout time.Time
	var record Record

	select {
	case record, _ = <-storage.records:
	case timeout = <-time.After(defaultTimeout):
	}
	test.Equals(t, Record{}, record, "read report")
	if timeout == emptyTime {
		test.FailOnError(t, fmt.Errorf("timeout didn't happen"))
	}
}

type parserMock struct{}

func (p *parserMock) Parse(l []byte) (Record, error) {
	line := string(l)
	log.Debug("parse: %v", line)
	if strings.HasPrefix(line, "pnc:") {
		panic(line)
	}
	if strings.HasPrefix(line, "err:") {
		return Record{}, errors.New(line)
	}
	return Record{Section: line}, nil
}

type storeMock struct {
	records chan Record
}

func (s *storeMock) Store(r Record) {
	s.records <- r
}

type logProviderMock struct {
	lines chan []byte
}

func (l *logProviderMock) Output() <-chan []byte {
	return l.lines
}
