package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	AppName          = "SyncWatcher"
	AppVersion       = "v0.6.7"
	AppAuthor        = "Yaott"
	Debug            = false
	SleepTime  uint8 = 1
)

var (
	ParamHelp       bool
	ParamVersion    bool
	ParamConfigFile string
	ParamLogFile    string
	ParamDebug      bool
	ParamIgnoreShow bool
	ParamSyncFirst  bool
)

var (
	Dir    string
	Script string
	Ignore []string
)

var (
	LogAllowSetFile = false
	SyncLock        = false
	LogFileTag      = false
	interruptTag    = false
)

func main() {
	SetupCloseHandler()
	flag.BoolVar(&ParamHelp, "h", false, "Help")
	flag.BoolVar(&ParamVersion, "v", false, "Version")
	flag.StringVar(&ParamConfigFile, "c", "config.json", "ConfigFile")
	flag.BoolVar(&ParamIgnoreShow, "is", false, "IgnoreShow")
	flag.BoolVar(&ParamSyncFirst, "s", false, "Sync Before Run")
	flag.StringVar(&ParamLogFile, "l", "", "LogFile")
	flag.BoolVar(&ParamDebug, "debug", false, "Debug")
	flag.Usage = usage
	flag.Parse()
	Debug = ParamDebug
	if ParamVersion {
		_, _ = fmt.Fprintln(os.Stdout, AppName+` version: `+AppVersion+` (Build From `+AppAuthor+`)`)
		return
	}
	if ParamHelp {
		usage()
		return
	}
	Log(0, AppName+` version: `+AppVersion+` (Build From `+AppAuthor+`)`)
	if Debug {
		Log(2, "Debug Mode")
	}
	Log(0, "Read Config...")
	ReadConfigFile(ParamConfigFile)
	Log(0, "Read Config Success")
	Log(0, "Start...")
	if ParamLogFile != "" {
		if !filepath.IsAbs(ParamLogFile) {
			var err error
			ParamLogFile, err = filepath.Abs(ParamLogFile)
			if err != nil {
				Log(-1, "LogFile Invalid")
			}
		}
		Log(0, "Redirect to Log File: "+ParamLogFile)
		LogAllowSetFile = true
	}
	w := new(fsnotify.Watcher)
	w, _ = fsnotify.NewWatcher()
	err := filepath.Walk(Dir, func(path string, info os.FileInfo, err error) error {
		pathre, _ := filepath.Rel(Dir, path)
		for _, v := range Ignore {
			match, err := regexp.MatchString(v, pathre)
			if err != nil {
				return err
			}
			if match {
				if ParamIgnoreShow {
					Log(1, "Ignore File(Dir): "+path)
				} else {
					Log(2, "Ignore File(Dir): "+path)
				}
				return nil
			}
		}
		if info.IsDir() {
			err = w.Add(path)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		Log(-1, "Watch Dir Fail")
	}
	SyncTag := make(chan string, 1024)
	go func() {
		for {
			select {
			case ev := <-w.Events:
				{
					RunTag := false
					switch ev.Op {
					case fsnotify.Write: // Write File
						RunTag = true
						Log(2, "写入文件: "+ev.Name)
					case fsnotify.Chmod: // Chmod File
						Log(2, "改变文件(夹)权限: "+ev.Name)
					case fsnotify.Create: // Create File
						RunTag = true
						Log(2, "创建文件(夹): "+ev.Name)
						file, err := os.Stat(ev.Name)
						if err == nil && file.IsDir() {
							err := w.Add(ev.Name)
							if err != nil {
								Log(-1, "Watching: Add Path Fail: ["+ev.Name+"] "+err.Error())
							}
							Log(2, "添加监控文件夹: "+ev.Name)
						} else if err != nil {
							Log(-1, "Watching: Check Path Fail: ["+ev.Name+"] "+err.Error())
						}
					case fsnotify.Remove: // Remove File
						RunTag = true
						Log(2, "删除文件(夹): "+ev.Name)
						_ = w.Remove(ev.Name)
						Log(2, "移除监控文件夹: "+ev.Name)
					case fsnotify.Rename: // Rename File
						RunTag = true
						Log(2, "重命名文件(夹): "+ev.Name)
						_ = w.Remove(ev.Name)
						Log(2, "移除监控文件夹: "+ev.Name)
					default:
					}
					if RunTag {
						SyncTag <- strings.ReplaceAll(uuid.New().String(), "-", "")
					}
				}
			case err := <-w.Errors:
				{
					Log(-1, "Watch Fail: "+err.Error())
				}
			}
		}
	}()
	func() {
		if ParamSyncFirst {
			Log(0, "Start to Sync...")
			RunSyncShell()
			Log(0, "Sync Finish!!")
		}
		Log(0, "Watching...")
		for {
			var DataSave string
			for {
				ContinueTag := false
				BreakTag := false
				select {
				case data := <-SyncTag:
					DataSave = data
					if len(SyncTag) == 0 {
						time.Sleep(1 * time.Second)
					}
					ContinueTag = true
					break
				default:
					if len(DataSave) != 0 {
						SyncLock = true
						Log(0, "Check File(Dir) Change...")
						Log(0, "Start to Sync...")
						RunSyncShell()
						Log(0, "Sync Finish!!")
						Log(0, "Watching...")
						SyncLock = false
						DataSave = ""
					}
					BreakTag = true
					break
				}
				if ContinueTag {
					continue
				} else if BreakTag {
					break
				}
			}
			time.Sleep(time.Duration(int64(SleepTime)) * time.Second)
		}
	}()
	return
}

func usage() {
	_, _ = fmt.Fprintf(os.Stdout, AppName+` version: `+AppVersion+` (Build From `+AppAuthor+`)
Usage: `+AppName+` [*]
   -c {string}  ConfigFile(default: config.json)
   -l {string}  LogFile
   -is          IgnoreShow
   -s           Sync Before Run
   -h           Show Help
   -v           Show Version
   -debug       Enable Debug
`)
}

func Log(Level int, Message string) {
	if LogAllowSetFile {
		if ParamLogFile != "" && !LogFileTag {
			_ = os.Remove(ParamLogFile)
			LogFile, err := os.OpenFile(ParamLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
			if err != nil {
				Log(-1, "LogFile Open Fail: "+err.Error())
			}
			log.SetOutput(LogFile)
			LogFileTag = true
		}
	}
	var LevelString string
	switch Level {
	case 0:
		LevelString = "Info"
	case -1:
		LevelString = "Fatal Error"
	case -2:
		LevelString = "Error"
	case 1:
		LevelString = "Warning"
	case 2:
		if !Debug {
			return
		}
		LevelString = "Debug"
	default:
		LevelString = "Unknown"
	}
	log.SetFlags(0)
	log.SetPrefix("[" + time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04:05") + "] ")
	MessageString := "[" + LevelString + "] " + Message
	if Level == -1 {
		log.Println(MessageString)
		Log(2, "Good Bye!!")
		os.Exit(-1)
	} else {
		log.Println(MessageString)
	}
}

func RunSyncShell() {
	var sout, serr bytes.Buffer
	cmd := exec.Command("bash", "-c", Script)
	Log(0, "Run Script: "+Script)
	cmd.Env = []string{"syncdir=" + Dir}
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	var waitStatus syscall.WaitStatus
	var retCode int
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			retCode = waitStatus.ExitStatus()
		} else {
			Log(-1, "Run Script Fail: "+err.Error())
		}
	} else {
		waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)
		retCode = waitStatus.ExitStatus()
	}
	Msg := `Run Script Finish: [` + strconv.Itoa(retCode) + `]` + "\n" + `Output: ` + "\n" + `==========` + "\n" + string(sout.Bytes()) + "\n" + `==========` + "\n" + `Error: ` + "\n" + `==========` + "\n" + string(serr.Bytes())
	Log(2, Msg)
}

func SetupCloseHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		if interruptTag {
			Log(0, "Program has started the end function")
		} else {
			interruptTag = true
			fmt.Println("[" + time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04:05") + "] [Warning] OS Interrupt")
			if SyncLock {
				fmt.Println("[" + time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04:05") + "] [Warning] Script Still Running... Interrupt!!")
			}
			fmt.Println("[" + time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04:05") + "] [Info] Good Bye!!")
			os.Exit(0)
		}
	}()
}

func ReadConfigFile(FileName string) {
	type ConfigStruct struct {
		Dir    string   `json:"dir"`
		Script string   `json:"script"`
		Ignore []string `json:"ignore"`
	}
	File, err := os.Open(FileName)
	if err != nil {
		Log(-1, "Config File Open Fail: "+err.Error())
		return
	}
	defer func(File *os.File) {
		_ = File.Close()
	}(File)
	ConfigData, err := ioutil.ReadAll(File)
	if err != nil {
		Log(-1, "Config File Read Fail: "+err.Error())
		return
	}
	ConfigParseData := &ConfigStruct{}
	err = json.Unmarshal(ConfigData, ConfigParseData)
	if err != nil {
		Log(-1, "Config File Parse Fail: "+err.Error())
		return
	}
	if ConfigParseData.Dir == "" {
		Log(-1, "Dir Not Found")
	} else {
		Dir = ConfigParseData.Dir
		var err error
		if !filepath.IsAbs(ConfigParseData.Dir) {
			Dir, err = filepath.Abs(ConfigParseData.Dir)
			if err != nil {
				Log(-1, "Dir Invalid")
			}
		}
		s, err := os.Stat(Dir)
		if err != nil {
			Log(-1, "Dir Open Fail")
		}
		if !s.IsDir() {
			Log(-1, "Dir Invalid")
		}
		Log(0, "Dir: "+Dir)
	}
	if ConfigParseData.Script == "" {
		Log(-1, "Script Not Found")
	} else {
		Script = ConfigParseData.Script
		if !filepath.IsAbs(Script) {
			var err error
			Script, err = filepath.Abs(Script)
			if err != nil {
				Log(-1, "Script Invalid")
			}
		}
		Log(0, "Script: "+Script)
	}
	if len(ConfigParseData.Ignore) > 0 {
		Ignore = ConfigParseData.Ignore
		Log(0, "Ignore List Add")
	}
}
