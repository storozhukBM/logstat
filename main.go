package main

import (
	"context"
	"github.com/storozhukBM/logstat/adapter/w3c"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/stat"
	"github.com/storozhukBM/logstat/watcher"
	"os"
	"os/signal"
	"syscall"
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

	storage, storageErr := stat.NewStatsStorage()
	if storageErr != nil {
		log.Error("%v", storageErr)
		return
	}

	_, adapterErr := w3c.NewW3CLogToStoreAdapter(fileWatcher, storage, 100)
	if adapterErr != nil {
		log.Error("%v", adapterErr)
		return
	}

	stopCh := make(chan os.Signal)
	defer close(stopCh)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)
	<-stopCh
}
