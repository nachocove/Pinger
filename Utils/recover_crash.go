package Utils

import (
	"github.com/nachocove/Pinger/Utils/Logging"
	"runtime"
)

func RecoverCrash(logger *Logging.Logger) error {
	if r := recover(); r != nil {
		stack := make([]byte, 8*1024)
		stack = stack[:runtime.Stack(stack, false)]
		logger.Error("Recovered Crash: %s\nStack: %s", r, stack)
		return r.(error)
	}
	return nil
}
