package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"regexp"
)

type ConfigReadStruct struct {
	Terminal      string                         `json:"terminal"`
	TerminalArg   string                         `json:"terminal_arg"`
	WatchSettings []ConfigReadWatchSettingStruct `json:"watch_settings"`
}

type ConfigReadWatchSettingStruct struct {
	Dir                string   `json:"dir"`
	Script             string   `json:"script"`
	Ignore             []string `json:"ignore"`
	SyncFirst          bool     `json:"sync_first"`
	IgnoreScriptOutput bool     `json:"ignore_script_output"`
}

type ConfigStruct struct {
	Terminal      string
	TerminalArg   string
	WatchSettings []ConfigWatchSettingStruct
}

type ConfigWatchSettingStruct struct {
	Dir                string
	Script             string
	Ignore             []*regexp.Regexp
	SyncFirst          bool
	IgnoreScriptOutput bool
}

func ReadConfig(FileName string) (ConfigStruct, error) {
	FileData, err := ioutil.ReadFile(FileName)
	if err != nil {
		return ConfigStruct{}, errors.New("read config file fail: " + err.Error())
	}
	var ConfigParse ConfigReadStruct
	var ConfigReal ConfigStruct
	if err = json.Unmarshal(FileData, &ConfigParse); err != nil {
		return ConfigStruct{}, errors.New("parse config file fail: " + err.Error())
	}
	if ConfigParse.Terminal != "" {
		ConfigReal.Terminal = ConfigParse.Terminal
		if ConfigParse.TerminalArg != "" {
			ConfigReal.TerminalArg = ConfigParse.TerminalArg
		} else {
			ConfigReal.TerminalArg = TerminalArgDefault
		}
	} else {
		ConfigReal.Terminal = TerminalDefault
	}
	ConfigReal.WatchSettings = make([]ConfigWatchSettingStruct, 0)
	if len(ConfigParse.WatchSettings) > 0 {
		var WatchSetting ConfigWatchSettingStruct
		for _, v := range ConfigParse.WatchSettings {
			d, err := os.Stat(v.Dir)
			if err != nil || os.IsNotExist(err) {
				return ConfigStruct{}, errors.New("`" + v.Dir + "` invalid: " + err.Error())
			}
			if !d.IsDir() {
				return ConfigStruct{}, errors.New("`" + v.Dir + "` invalid")
			}
			WatchSetting.Dir = v.Dir
			f, err := os.Stat(v.Script)
			if err != nil {
				return ConfigStruct{}, errors.New("`" + v.Script + "` invalid: " + err.Error())
			}
			if f.IsDir() {
				return ConfigStruct{}, errors.New("`" + v.Script + "` invalid")
			}
			WatchSetting.Script = v.Script
			WatchSetting.Ignore = make([]*regexp.Regexp, 0)
			for _, r := range v.Ignore {
				Regexp, err := regexp.Compile(r)
				if err != nil {
					return ConfigStruct{}, errors.New("regexp `" + r + "` invalid: " + err.Error())
				}
				WatchSetting.Ignore = append(WatchSetting.Ignore, Regexp)
			}
			WatchSetting.SyncFirst = v.SyncFirst
			WatchSetting.IgnoreScriptOutput = v.IgnoreScriptOutput
			ConfigReal.WatchSettings = append(ConfigReal.WatchSettings, WatchSetting)
		}
	} else {
		return ConfigStruct{}, errors.New("`watch_settings` is nil")
	}
	return ConfigReal, nil
}
