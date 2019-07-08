package watcher

import (
	"context"
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"io"
	"math/rand"
	"runtime/debug"
	"time"
)

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
*/
type LogFileWatcher struct {
	ctx           context.Context
	pollPeriod    time.Duration
	backOffRandom *rand.Rand

	reader *fileReader

	output chan []byte
}

func NewLogFileWatcher(ctx context.Context, fileName string, readerBufSize uint, pollPeriod time.Duration) (*LogFileWatcher, error) {
	if fileName == "" {
		return nil, fmt.Errorf("fileName can't be empty")
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("ctx is already closed")
	}
	reader, readerErr := newFileReader(fileName, readerBufSize)
	if readerErr != nil {
		return nil, readerErr
	}
	result := &LogFileWatcher{
		ctx:           ctx,
		pollPeriod:    pollPeriod,
		backOffRandom: rand.New(rand.NewSource(time.Now().Unix())),
		reader:        reader,
		output:        make(chan []byte),
	}
	go result.run()
	return result, nil
}

func (l *LogFileWatcher) Output() <-chan []byte {
	return l.output
}

func (l *LogFileWatcher) run() {
	defer func() {
		closeErr := l.reader.close()
		if closeErr != nil {
			log.Error("can't close file reader: %v", closeErr)
		}
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
	defer panicHandle()
	for {
		slice, readErr := l.reader.readOneLineAsSlice()
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

func panicHandle() {
	r := recover()
	if r != nil {
		log.Error("panic happened: %+v", r)
		debug.PrintStack()
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
