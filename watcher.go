package main

import (
	"github.com/fsnotify/fsnotify"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

func WatcherRun(CFG *ConfigWatchSettingStruct, Terminal, TerminalArg string) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		Log(-1, err)
		MainCancel()
		return
	}
	err = filepath.Walk(CFG.Dir, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			return nil
		}
		if len(CFG.Ignore) > 0 {
			PathRe, _ := filepath.Rel(CFG.Dir, path)
			var (
				IgnoreWorkGroup sync.WaitGroup
				IgnoreLock      sync.Mutex
			)
			for _, v := range CFG.Ignore {
				IgnoreWorkGroup.Add(1)
				go func(regx *regexp.Regexp) {
					defer IgnoreWorkGroup.Done()
					if regx.MatchString(PathRe) {
						IgnoreLock.TryLock()
					}
				}(v)
			}
			IgnoreWorkGroup.Wait()
			if !IgnoreLock.TryLock() {
				Log(0, "Ignore", path)
				IgnoreLock.Unlock()
				return nil
			}
		}
		err = w.Add(path)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		Log(-1, "watch prepare fail:", err)
		MainCancel()
		return
	}
	if CFG.SyncFirst {
		Log(0, "Run First:", CFG.Script)
		func() {
			Env := make(map[string]string)
			Env["syncdir"] = CFG.Dir
			Stdout, Stderr, err := CommandRun(CFG.Script, Terminal, TerminalArg, Env)
			StdoutBytes, _ := ioutil.ReadAll(Stdout)
			StderrBytes, _ := ioutil.ReadAll(Stderr)
			if err != nil {
				Log(-2, "run command `"+CFG.Script+"` fail:", err, "Stdout:", string(StdoutBytes), "Stderr:", string(StderrBytes))
			} else {
				if !CFG.IgnoreScriptOutput {
					Log(2, "run command `"+CFG.Script+"`", "Stdout:", string(StdoutBytes), "Stderr:", string(StderrBytes))
				}
			}
		}()
		Log(0, "Run First:", CFG.Script, "Finish")
	}
	RunLock := sync.Mutex{}
	Log(0, "Watching...")
	for {
		select {
		case event := <-w.Events:
			RunTag := false
			switch event.Op {
			case fsnotify.Write: // Write File
				RunTag = true
				Log(2, "write file:", event.Name)
			case fsnotify.Chmod: // Chmod File
				Log(2, "chmod file(dir):", event.Name)
			case fsnotify.Create: // Create File
				RunTag = true
				Log(2, "create file(dir):", event.Name)
				file, err := os.Stat(event.Name)
				if err == nil && file.IsDir() {
					err := w.Add(event.Name)
					if err != nil {
						Log(-2, "watcher: add dir fail: ["+event.Name+"]", err)
					} else {
						Log(2, "watcher add dir:", event.Name)
					}
				} else if err != nil {
					Log(-2, "watcher: check path fail: ["+event.Name+"]", err)
				}
			case fsnotify.Remove: // Remove File
				RunTag = true
				Log(2, "delete file(dir):", event.Name)
				err = w.Remove(event.Name)
				if err != nil {
					Log(-2, "watcher: delete dir fail:", event.Name)
				} else {
					Log(2, "watcher: delete dir:", event.Name)
				}
			case fsnotify.Rename: // Rename File
				RunTag = true
				Log(2, "rename file(dir):", event.Name)
				err = w.Remove(event.Name)
				if err != nil {
					Log(-2, "watcher: delete dir fail:", event.Name)
				} else {
					Log(2, "watcher: delete dir:", event.Name)
				}
			default:
			}
			if RunTag {
				switch len(w.Events) {
				case 0:
					if RunLock.TryLock() {
						go func() {
							defer RunLock.Unlock()
							Log(0, "Run Script", CFG.Script)
							Env := make(map[string]string)
							Env["syncdir"] = CFG.Dir
							Stdout, Stderr, err := CommandRun(CFG.Script, Terminal, TerminalArg, Env)
							StdoutBytes, _ := ioutil.ReadAll(Stdout)
							StderrBytes, _ := ioutil.ReadAll(Stderr)
							if err != nil {
								Log(-2, "run command `"+CFG.Script+"` fail:", err, "Stdout:", string(StdoutBytes), "Stderr:", string(StderrBytes))
							} else {
								Log(2, "run command `"+CFG.Script+"`", "Stdout:", string(StdoutBytes), "Stderr:", string(StderrBytes))
							}
							Log(0, "Run Script", CFG.Script, "Finish")
						}()
					}
				case 1:
					<-time.After(1 * time.Second)
					continue
				default:
					continue
				}
			}
		case <-MainCtx.Done():
			return
		}
	}
}
