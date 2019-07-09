package pnc

import (
	"github.com/storozhukBM/logstat/common/log"
	"runtime/debug"
)

func PanicHandle() {
	r := recover()
	if r != nil {
		log.Error("panic happened: %+v", r)
		debug.PrintStack()
	}
}
