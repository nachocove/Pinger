package Utils

import (
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"
)

type SignalDebugToggle interface {
	ToggleDebug()
}

var usr1Channel chan os.Signal
var usr1Signalhandlers []SignalDebugToggle
var usr1Mutex *sync.Mutex
var usr1Initialized bool

func init() {
	usr1Mutex = &sync.Mutex{}
}
func initUsr1Catcher() {
	usr1Mutex.Lock()
	defer usr1Mutex.Unlock()
	if !usr1Initialized {
		usr1Channel = make(chan os.Signal, 1)
		usr1Signalhandlers = make([]SignalDebugToggle, 0)
		signal.Notify(usr1Channel, syscall.SIGUSR1)
		go usr1Catcher()
		usr1Initialized = true
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
			fmt.Fprintf(os.Stderr, "Received unexpected signal %s\n", signal.String())
		}
	}
}

var usr2Channel chan os.Signal
var usr2Mutex *sync.Mutex
var usr2Initialized bool

func init() {
	usr2Mutex = &sync.Mutex{}
}
func InitCpuProfileSignal() {
	usr2Mutex.Lock()
	defer usr2Mutex.Unlock()
	if !usr2Initialized {
		usr2Channel = make(chan os.Signal, 1)
		signal.Notify(usr2Channel, syscall.SIGUSR2)
		go usr2Catcher()
		usr2Initialized = true
	}
}

func usr2Catcher() {
	cpuprofile := ""
	memprofile := ""
	for {
		signal := <-usr2Channel
		switch {
		case signal == syscall.SIGUSR2:
			usr2Mutex.Lock()
			defer usr2Mutex.Unlock()
			if cpuprofile == "" {
				now := time.Now().Local()
				cpuprofile = fmt.Sprintf("%s-%s.cpuprof", path.Base(os.Args[0]), now.Format("20060102150405"))
				memprofile = fmt.Sprintf("%s-%s.memprof", path.Base(os.Args[0]), now.Format("20060102150405"))
				go func() {
					defer func() {
						usr2Mutex.Lock()
						defer usr2Mutex.Unlock()
						cpuprofile = ""
						memprofile = ""
					}()
					m, err := os.Create(memprofile)
					if err != nil {
						fmt.Fprintf(os.Stderr, "ERROR: Could not open memprof file %s", memprofile)
						return
					}
					defer m.Close()
					f, err := os.Create(cpuprofile)
					if err != nil {
						fmt.Fprintf(os.Stderr, "ERROR: Could not open cpuprof file %s", cpuprofile)
						return
					}
					pprof.StartCPUProfile(f)
					time.Sleep(time.Duration(60) * time.Second)
					pprof.WriteHeapProfile(m)
					defer func() {
						pprof.StopCPUProfile()
						f.Close()
					}()
				}()
			}
		default:
			fmt.Fprintf(os.Stderr, "Received unexpected signal %s\n", signal.String())
		}
	}
}
