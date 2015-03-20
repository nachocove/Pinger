package Utils

import (
	"github.com/nachocove/Pinger/Utils/Logging"
	"runtime"
)

func RecoverCrash(logger *Logging.Logger) {
	if err := recover(); err != nil {
		logger.Error("Error: %s", err)
		stack := make([]byte, 8*1024)
		stack = stack[:runtime.Stack(stack, false)]
		logger.Debug("Stack: %s", stack)
	}
}

