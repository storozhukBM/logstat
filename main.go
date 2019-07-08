package main

import (
	"context"
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/watcher"
	"os"
	"runtime"
	"time"
)

func main() {
	fileWatcher, watcherErr := watcher.NewLogFileWatcher(
		context.Background(),
		"/tmp/access.log",
		16*1024,
		100*time.Millisecond,
	)
	if watcherErr != nil {
		log.Error("%v", watcherErr)
		os.Exit(-1)
	}
	go func() {
		for {
			time.Sleep(2 * time.Second)
			printMemUsage()
		}
	}()
	count := 0
	for line := range fileWatcher.Output() {
		count += len(line)
		if count%100 == 34 {
			fmt.Printf("%v", count)
		}
	}
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
