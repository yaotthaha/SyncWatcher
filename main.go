package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	AppName    = "SyncWatcher"
	AppVersion = "v1.0.0-build-6"
	AppAuthor  = "Yaott"
)

const (
	TerminalDefault    = "bash"
	TerminalArgDefault = "-c"
)

var (
	MainCtx    context.Context
	MainCancel context.CancelFunc
	Logger     *log.Logger
	LogFile    *os.File
)

var Params struct {
	Help    bool
	Version bool
	Debug   bool
	Config  string
	LogFile string
	Start   bool
}

func init() {
	flag.BoolVar(&Params.Help, "h", false, "Show Help")
	flag.BoolVar(&Params.Version, "v", false, "Show Version")
	flag.BoolVar(&Params.Debug, "debug", false, "Show Debug Log")
	flag.StringVar(&Params.Config, "c", "./config.json", "Set Config File")
	flag.StringVar(&Params.LogFile, "log", "", "Set Log Output File")
	flag.Parse()
	if Params.Version {
		_, _ = fmt.Fprintln(os.Stdout, AppName, AppVersion, "(Build From "+AppAuthor+")")
		return
	}
	if Params.Help {
		flag.Usage()
		return
	}
	Logger = &log.Logger{}
	Logger.SetPrefix("")
	Logger.SetFlags(0)
	Logger.SetOutput(os.Stdout)
	Params.Start = true
	go SetupCloseHandler()
}

func main() {
	if !Params.Start {
		return
	}
	Log(0, AppName, AppVersion, "(Build From "+AppAuthor+")")
	Log(0, "Start...")
	Log(0, "Read Config", Params.Config)
	CFG, err := ReadConfig(Params.Config)
	if err != nil {
		Log(-1, err)
		return
	}
	Log(0, "Read Config Success")
	if Params.LogFile != "" {
		LogFile, err = os.OpenFile(Params.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
		if err != nil {
			Log(-1, "Set Log File Fail:", err)
			return
		}
		defer func(LogFile *os.File) {
			_ = LogFile.Close()
		}(LogFile)
		Log(0, "Redirect Log to File:", Params.LogFile)
		Logger.SetOutput(LogFile)
	}
	MainCtx, MainCancel = context.WithCancel(context.Background())
	Log(0, "Run...")
	var WorkGroup sync.WaitGroup
	for _, v := range CFG.WatchSettings {
		WorkGroup.Add(1)
		go func(cfg *ConfigWatchSettingStruct) {
			defer WorkGroup.Done()
			WatcherRun(cfg, CFG.Terminal, CFG.TerminalArg)
		}(&v)
	}
	WorkGroup.Wait()
	Log(0, "Good Bye!!")
}

func Log(Level int, Message ...interface{}) {
	LevelStr := "Unknown"
	switch Level {
	case -2:
		LevelStr = "Error"
	case -1:
		LevelStr = "Fatal Error"
	case 0:
		LevelStr = "Info"
	case 1:
		LevelStr = "Warning"
	case 2:
		if !Params.Debug {
			return
		}
		LevelStr = "Debug"
	}
	MessageStr := "[" + time.Now().Format("2006-01-02 15:04:05") + "] [" + LevelStr + "] "
	for _, v := range Message {
		MessageStr += fmt.Sprintf("%v ", v)
	}
	MessageStr = MessageStr[:len(MessageStr)-1]
	Logger.Println(MessageStr)
}

func SetupCloseHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	Log(1, "OS Interrupt")
	MainCancel()
}
