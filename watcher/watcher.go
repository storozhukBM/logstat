package watcher

import (
	"context"
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/common/pnc"
	"io"
	"math/rand"
	"time"
)

type lineReader interface {
	ReadOneLineAsSlice() ([]byte, error)
}

/*
Component used to stream new lines from file to output channel of bytes.
It starts separate goroutine to track file changes.

Responsibilities:
 - watch for file and its new lines
 - gracefully react if file don't exists, is empty or has no new lines
 - push new lines to un-buffered output channel

Attention:
 - output channel contains a view to internal reading buffer
to avoid copying and pressure on GC. This view is only valid before the next
line is fetched from channel. If you need some parts of it to remain accessible,
copy required parts.
- you should cancel associated context to free all attached resources.

Future:
This solution is too straightforward and can have bad latency and energy efficiency
capabilities. In future it can be replace by some specialized library with usage of
fsnotify.
*/
type LogFileWatcher struct {
	ctx           context.Context
	pollPeriod    time.Duration
	backOffRandom *rand.Rand
	reader        lineReader

	output chan []byte
}

func NewLogFileWatcher(ctx context.Context, reader lineReader, pollPeriod time.Duration) (*LogFileWatcher, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("ctx is already closed")
	}
	if reader == nil {
		return nil, fmt.Errorf("reader can't be nil")
	}
	result := &LogFileWatcher{
		ctx:           ctx,
		pollPeriod:    pollPeriod,
		backOffRandom: rand.New(rand.NewSource(time.Now().Unix())),
		reader:        reader,

		output: make(chan []byte),
	}
	go result.run()
	return result, nil
}

func (l *LogFileWatcher) Output() <-chan []byte {
	return l.output
}

func (l *LogFileWatcher) run() {
	defer func() {
		log.Debug("closing watcher channel")
		close(l.output)
	}()

	for l.ctx.Err() == nil {
		cycleErr := l.cycle()
		if cycleErr != nil {
			log.Error("error happened: %v", cycleErr)
			l.waitOnError()
		} else {
			l.wait()
		}
	}
}

func (l *LogFileWatcher) cycle() error {
	defer pnc.PanicHandle()
	for {
		slice, readErr := l.reader.ReadOneLineAsSlice()
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
		select {
		case l.output <- slice:
		case <-l.ctx.Done():
			return l.ctx.Err()
		}
	}
}

func (l *LogFileWatcher) wait() {
	select {
	case <-time.After(l.pollPeriod):
	case <-l.ctx.Done():
	}
}

func (l *LogFileWatcher) waitOnError() {
	multiplier := time.Duration(l.backOffRandom.Intn(8) + 2)
	select {
	case <-time.After(multiplier * l.pollPeriod):
	case <-l.ctx.Done():
	}
}
