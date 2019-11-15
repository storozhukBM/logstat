package main

import (
	"context"
	"fmt"
	"github.com/storozhukBM/logstat/alert"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/config"
	"github.com/storozhukBM/logstat/file"
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
	cfg := config.ParseFlagsAsConfig()
	if cfg.DebugMode {
		log.GlobalDebugEnabled = true
	}

	fileReader, readerErr := file.NewReader(cfg.FileName, cfg.FileReadBufSizeInBytes)
	if readerErr != nil {
		log.WithError(readerErr, "can't setup file reader")
		return
	}
	defer log.OnError(fileReader.Close, "can't close file reader")

	applicationCtx, applicationCancel := context.WithCancel(context.Background())
	defer applicationCancel()

	parser, parserErr := w3c.NewLineToStoreRecordParser(cfg.W3CParserSectionsStringCacheSize)
	if parserErr != nil {
		log.WithError(parserErr, "can't setup w3c log parser")
		return
	}

	storage, storageErr := stat.NewStorage(10, 10)
	if storageErr != nil {
		log.WithError(storageErr, "can't setup traffic aggregation storage")
		return
	}

	_, watcherErr := watcher.NewLogFileWatcher(applicationCtx, fileReader, storage, parser, cfg.FileReadPollPeriod)
	if watcherErr != nil {
		log.WithError(watcherErr, "can't setup file watcher")
		return
	}

	trafficAlert, trafficAlertErr := alert.NewTrafficState(
		120, 10, 10, 10,
	)
	if trafficAlertErr != nil {
		log.WithError(trafficAlertErr, "can't setup traffic alert")
		return
	}

	stdOutView, viewErr := view.NewIOView(applicationCtx, 10*time.Second, os.Stdout)
	if viewErr != nil {
		log.WithError(viewErr, "can't setup io view")
		return
	}

	_, alertSubscriptionErr := alert.NewAlertsSubscription(trafficAlert, stdOutView.TrafficAlert)
	if alertSubscriptionErr != nil {
		log.WithError(alertSubscriptionErr, "can't setup alert broadcast")
		return
	}

	_, statReportSubscriptionErr := stat.NewReportSubscription(storage, trafficAlert.Store, stdOutView.Report)
	if statReportSubscriptionErr != nil {
		log.WithError(statReportSubscriptionErr, "can't setup traffic reports broadcast")
		return
	}

	stopCh := make(chan os.Signal, 1)
	defer close(stopCh)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)
	<-stopCh
	fmt.Println()
}
