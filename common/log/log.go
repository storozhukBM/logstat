package log

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"time"
)

func OnError(errFunc func() error, format string, args ...interface{}) func() {
	return func() {
		err := errFunc()
		if err != nil {
			Error("%s: %v", fmt.Sprintf(format, args...), err)
		}
	}
}

func WithError(err error, format string, args ...interface{}) {
	fmt.Print("[ERROR] ")
	_, fn, line, _ := runtime.Caller(1)
	fmt.Printf("%s:%d - ", fn, line)
	fmt.Printf(format, args...)
	fmt.Printf(": %v", err)
	fmt.Println()
}

func Error(format string, args ...interface{}) {
	fmt.Print("[ERROR] ")
	_, fn, line, _ := runtime.Caller(1)
	fmt.Printf("%s:%d - ", fn, line)
	fmt.Printf(format, args...)
	fmt.Println()
}

var GlobalDebugEnabled = false

func Debug(format string, args ...interface{}) {
	if !GlobalDebugEnabled {
		return
	}
	fmt.Print("[DEBUG] ")
	_, fn, line, _ := runtime.Caller(1)
	fmt.Printf("%s:%d - ", fn, line)
	fmt.Printf(format, args...)
	fmt.Println()
}

func PrintMemoryStatsInBackground() {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			if GlobalDebugEnabled {
				printMemUsage()
			}
		}
	}()
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
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
