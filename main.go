package main

import (
	"context"
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/watcher"
	"time"
)

func main() {
	log.GlobalDebugEnabled = true
	log.PrintMemoryStatsInBackground()

	fileReader, readerErr := watcher.NewFileReader("/tmp/access.log", 16*1024)
	if readerErr != nil {
		log.WithError(readerErr, "can't setup file reader")
		return
	}
	defer log.OnError(fileReader.Close, "can't close file reader")

	applicationCtx, applicationCancel := context.WithCancel(context.Background())
	defer applicationCancel()

	fileWatcher, watcherErr := watcher.NewLogFileWatcher(applicationCtx, fileReader, 100*time.Millisecond)
	if watcherErr != nil {
		log.Error("%v", watcherErr)
		return
	}
	count := 0
	for line := range fileWatcher.Output() {
		count += len(line) % 10
		if count%10 == 0 {
			fmt.Printf("%v\n", count)
		}
	}
}
