package stat

import (
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/common/pnc"
)

type logSource interface {
	Output() <-chan []byte
}

type store interface {
	Store(r Record)
}

type parser interface {
	Parse(line []byte) (Record, error)
}

type LogToStoreAdapter struct {
	lineProvider logSource
	store        store
	parser       parser
}

func NewLogToStoreAdapter(log logSource, store store, parser parser) (*LogToStoreAdapter, error) {
	if log == nil {
		return nil, fmt.Errorf("log can't be nil")
	}
	if store == nil {
		return nil, fmt.Errorf("store can't be nil")
	}
	if parser == nil {
		return nil, fmt.Errorf("parser can't be nil")
	}
	if log.Output() == nil {
		return nil, fmt.Errorf("log output can't be nil")
	}
	result := &LogToStoreAdapter{
		lineProvider: log,
		store:        store,
		parser:       parser,
	}
	go result.run()
	return result, nil
}

func (a *LogToStoreAdapter) run() {
	input := a.lineProvider.Output()
	for line := range input {
		// move to separate function for panic handling
		func() {
			defer pnc.PanicHandle()
			record, parseErr := a.parser.Parse(line)
			if parseErr != nil {
				log.WithError(parseErr, "can't parse line")
				return
			}
			a.store.Store(record)
		}()
	}
}
