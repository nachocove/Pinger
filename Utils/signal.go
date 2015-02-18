package Utils

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type SignalDebugToggle interface {
	ToggleDebug()
}

var usr1Channel chan os.Signal
var usr1Signalhandlers []SignalDebugToggle
var usr1Mutex *sync.Mutex
var usr1MutexInitialized bool

func init() {
	usr1Mutex = &sync.Mutex{}
}
func initUsr1Catcher() {
	usr1Mutex.Lock()
	defer usr1Mutex.Unlock()
	if !usr1MutexInitialized {
		usr1Channel = make(chan os.Signal, 1)
		usr1Signalhandlers = make([]SignalDebugToggle, 0)
		signal.Notify(usr1Channel, syscall.SIGUSR1)
		go usr1Catcher()
		usr1MutexInitialized = true
	}
}

func AddDebugToggleSignal(f SignalDebugToggle) {
	initUsr1Catcher()
	usr1Signalhandlers = append(usr1Signalhandlers, f)
}

func usr1Catcher() {
	for {
		signal := <-usr1Channel
		switch {
		case signal == syscall.SIGUSR1:
			for _, f := range usr1Signalhandlers {
				f.ToggleDebug()
			}
		default:
			fmt.Fprintf(os.Stderr, "Received unexpected signal %d\n", signal)
		}
	}
}
