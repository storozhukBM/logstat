package watcher

import (
	"context"
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/common/pnc"
	"github.com/storozhukBM/logstat/stat"
	"io"
	"math/rand"
	"time"
)

type lineReader interface {
	ReadOneLineAsSlice() ([]byte, error)
}

type storage interface {
	Store(r stat.Record)
}

type parser interface {
	Parse(line []byte) (stat.Record, error)
}

/*
A component used to stream new lines from file, parse and store them.
It starts separate goroutine to track file changes.

Responsibilities:
	- watch for file and its new lines
	- gracefully react if the file doesn't exist, is empty or has no new lines
	- push new lines to the provided parser
	- feed parsed log record to storage.

Attention:
	- you should cancel associated context to free all attached resources.

Future:
	- this solution is too straightforward and can have bad latency and energy efficiency
	capabilities. In the future, it can be replaced by some specialized library with the
	usage of fsnotify.
*/
type LogFileWatcher struct {
	ctx           context.Context
	pollPeriod    time.Duration
	backOffRandom *rand.Rand
	reader        lineReader
	storage       storage
	parser        parser
}

func NewLogFileWatcher(ctx context.Context, reader lineReader, store storage, parser parser, pollPeriod time.Duration) (*LogFileWatcher, error) {
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
		storage:       store,
		parser:        parser,
	}
	go result.run()
	return result, nil
}

func (l *LogFileWatcher) run() {
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
	for l.ctx.Err() == nil {
		slice, readErr := l.reader.ReadOneLineAsSlice()
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}

		record, parseErr := l.parser.Parse(slice)
		if parseErr != nil {
			// we should immediately proceed with next line
			log.Error("parser error happened: %v", parseErr)
			continue
		}

		l.storage.Store(record)
	}
	return nil
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
