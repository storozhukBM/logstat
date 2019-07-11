package watcher

import (
	"context"
	"fmt"
	"github.com/storozhukBM/logstat/common/test"
	"github.com/storozhukBM/logstat/stat"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogFileWatcher(t *testing.T) {
	t.Parallel()
	reader := newFileReaderMock()
	store := newStorageMock()
	parser := &parserMock{}
	ctx := context.Background()

	_, watcherErr := NewLogFileWatcher(ctx, reader, store, parser, 5*time.Millisecond)
	test.FailOnError(t, watcherErr)
	waitForRecordTimeout(t, store)

	reader.lines <- []byte("first1")
	waitForRecord(t, store, "first1")
	reader.lines <- []byte("second")
	waitForRecord(t, store, "second")

	waitForRecordTimeout(t, store)

	reader.lines <- []byte("first2")
	reader.lines <- []byte("second")
	waitForRecord(t, store, "first2")
	waitForRecord(t, store, "second")

	waitForRecordTimeout(t, store)

	reader.setError(io.ErrUnexpectedEOF)
	waitForRecordTimeout(t, store)

	reader.setError(nil)
	waitForRecordTimeout(t, store)

	reader.lines <- []byte("first3")
	waitForRecord(t, store, "first3")

	reader.lines <- []byte("first4")
	waitForRecord(t, store, "first4")

	reader.lines <- []byte("err: parser failure")
	waitForRecordTimeout(t, store)

	reader.lines <- []byte("first5")
	waitForRecord(t, store, "first5")

	reader.lines <- []byte("pnc: expected panic for tests")
	waitForRecordTimeout(t, store)

	reader.lines <- []byte("first6")
	waitForRecord(t, store, "first6")
}

type parserMock struct{}

func (p *parserMock) Parse(line []byte) (stat.Record, error) {
	lineStr := string(line)
	if strings.HasPrefix(lineStr, "err:") {
		return stat.Record{}, fmt.Errorf("can't parse line: %v", lineStr)
	}
	if strings.HasPrefix(lineStr, "pnc:") {
		panic(fmt.Errorf("can't parse line: %v", lineStr))
	}
	return stat.Record{Section: lineStr}, nil
}

type storageMock struct {
	records chan stat.Record
}

func newStorageMock() *storageMock {
	result := &storageMock{
		records: make(chan stat.Record, 100),
	}
	return result
}

func (p *storageMock) Store(r stat.Record) {
	p.records <- r
}

func waitForRecord(t *testing.T, storage *storageMock, expSection string) {
	{
		var timeout time.Time
		var record stat.Record
		var open bool
		select {
		case record, open = <-storage.records:
		case timeout = <-time.After(time.Second):
		}
		test.Equals(t, time.Time{}, timeout, "no timeout should happen")
		test.Equals(t, expSection, record.Section, "read record mismatch")
		test.Equals(t, true, open, "ring shouldn't be closed")
	}
}

func waitForRecordTimeout(t *testing.T, storage *storageMock) {
	emptyTime := time.Time{}

	var timeout time.Time
	var record stat.Record

	select {
	case record, _ = <-storage.records:
	case timeout = <-time.After(10 * time.Millisecond):
	}
	test.Equals(t, stat.Record{}, record, "read report")
	if timeout == emptyTime {
		test.FailOnError(t, fmt.Errorf("timeout didn't happen"))
	}
}

type fileReaderMock struct {
	mu    sync.Mutex
	lines chan []byte
	pnc   bool
	err   error
}

func newFileReaderMock() *fileReaderMock {
	result := &fileReaderMock{
		lines: make(chan []byte, 100),
	}
	return result
}

func (r *fileReaderMock) setError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *fileReaderMock) setPanic(p bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pnc = p
}

func (r *fileReaderMock) ReadOneLineAsSlice() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pnc {
		panic("expected panic for tests")
	}
	if r.err != nil {
		return nil, r.err
	}
	if len(r.lines) == 0 {
		return nil, io.EOF
	}
	return <-r.lines, nil
}
