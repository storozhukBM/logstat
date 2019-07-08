package log

import (
	"fmt"
	"runtime"
)

func OnError(errFunc func() error, format string, args ...interface{}) func() {
	return func() {
		err := errFunc()
		if err != nil {
			Error("%s: %v", fmt.Sprintf(format, args...), err)
		}
	}
}

func Error(format string, args ...interface{}) {
	fmt.Print("[ERROR] ")
	_, fn, line, _ := runtime.Caller(1)
	fmt.Printf("%s:%d - ", fn, line)
	fmt.Printf(format, args...)
	fmt.Println()
}

func Debug(format string, args ...interface{}) {
	fmt.Print("[DEBUG] ")
	_, fn, line, _ := runtime.Caller(1)
	fmt.Printf("%s:%d - ", fn, line)
	fmt.Printf(format, args...)
	fmt.Println()
}
