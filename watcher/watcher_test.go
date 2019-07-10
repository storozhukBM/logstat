package watcher

import (
	"context"
	"fmt"
	"github.com/storozhukBM/logstat/common/test"
	"io"
	"sync"
	"testing"
	"time"
)

const defaultTimeout = 15 * time.Millisecond

func TestLogFileWatcher(t *testing.T) {
	t.Parallel()
	reader := newFileReaderMock()
	ctx, ctxCancel := context.WithCancel(context.Background())

	watcher, watcherErr := NewLogFileWatcher(ctx, reader, time.Millisecond)
	test.FailOnError(t, watcherErr)
	test.Equals(t, 0, len(watcher.Output()), "watcher should be empty")
	waitTillTimeout(t, watcher)

	reader.lines <- []byte("first")
	reader.lines <- []byte("second")
	waitForLine(t, watcher, []byte("first"))
	waitForLine(t, watcher, []byte("second"))

	reader.setError(io.ErrUnexpectedEOF)
	waitTillTimeout(t, watcher)

	reader.setError(nil)
	waitTillTimeout(t, watcher)

	ctxCancel()
	time.Sleep(defaultTimeout)
	_, watcherOpen := <-watcher.Output()
	test.Equals(t, false, watcherOpen, "watcher should be closed with ctx")
}

func TestLogFileWatcherCloseChannelOnWriteBlock(t *testing.T) {
	reader := newFileReaderMock()
	ctx, ctxCancel := context.WithCancel(context.Background())

	watcher, watcherErr := NewLogFileWatcher(ctx, reader, time.Millisecond)
	test.FailOnError(t, watcherErr)
	test.Equals(t, 0, len(watcher.Output()), "watcher should be empty")
	waitTillTimeout(t, watcher)

	reader.lines <- []byte("first")
	ctxCancel()
	time.Sleep(defaultTimeout)
	_, watcherOpen := <-watcher.Output()
	test.Equals(t, false, watcherOpen, "watcher should be closed with ctx")
}

func TestLogFileWatcherHandlePanic(t *testing.T) {
	reader := newFileReaderMock()
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	watcher, watcherErr := NewLogFileWatcher(ctx, reader, time.Millisecond)
	test.FailOnError(t, watcherErr)
	test.Equals(t, 0, len(watcher.Output()), "watcher should be empty")
	waitTillTimeout(t, watcher)

	reader.setPanic(true)
	waitTillTimeout(t, watcher)
	reader.setPanic(false)

	reader.lines <- []byte("first")
	waitForLine(t, watcher, []byte("first"))
}

func waitForLine(t *testing.T, watcher *LogFileWatcher, line []byte) {
	{
		var timeout time.Time
		var bytes []byte
		var open bool
		select {
		case bytes, open = <-watcher.Output():
		case timeout = <-time.After(defaultTimeout):
		}
		test.Equals(t, time.Time{}, timeout, "no timeout should happen")
		test.Equals(t, line, bytes, "read line")
		test.Equals(t, true, open, "watcher shouldn't be closed")
	}
}

func waitTillTimeout(t *testing.T, watcher *LogFileWatcher) {
	emptyTime := time.Time{}

	var timeout time.Time
	var bytes []byte
	select {
	case bytes, _ = <-watcher.Output():
	case timeout = <-time.After(defaultTimeout):
	}
	test.Equals(t, []byte(nil), bytes, "read line")
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
