package main

import (
	"context"
	"github.com/storozhukBM/logstat/alert"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/parser/w3c"
	"github.com/storozhukBM/logstat/stat"
	"github.com/storozhukBM/logstat/view"
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

	parser, parserErr := w3c.NewLineToStoreRecordParser(100)
	if parserErr != nil {
		log.Error("%v", parserErr)
		return
	}

	storage, storageErr := stat.NewStorage(10, 10)
	if storageErr != nil {
		log.Error("%v", storageErr)
		return
	}
	_, adapterErr := stat.NewLogToStoreAdapter(fileWatcher, storage, parser)
	if adapterErr != nil {
		log.Error("%v", adapterErr)
		return
	}

	trafficAlert, trafficAlertErr := alert.NewTrafficState(
		120, 10, 10, 10,
	)
	if trafficAlertErr != nil {
		log.Error("%v", trafficAlertErr)
		return
	}

	stdOutView, viewErr := view.NewIOView(applicationCtx, 10*time.Second, os.Stdout)
	if viewErr != nil {
		log.Error("%v", viewErr)
		return
	}

	_, alertSubscriptionErr := alert.NewAlertsSubscription(trafficAlert, stdOutView.TrafficAlert)
	if alertSubscriptionErr != nil {
		log.Error("%v", alertSubscriptionErr)
		return
	}

	_, statReportSubscriptionErr := stat.NewReportSubscription(storage, trafficAlert.Store, stdOutView.Report)
	if statReportSubscriptionErr != nil {
		log.Error("%v", statReportSubscriptionErr)
		return
	}

	stopCh := make(chan os.Signal)
	defer close(stopCh)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)
	<-stopCh
}
