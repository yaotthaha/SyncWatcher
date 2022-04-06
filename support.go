package main

import (
	"bytes"
	"io"
	"os/exec"
)

func CommandRun(Command string, Terminal, TerminalArg string, Env map[string]string) (io.Reader, io.Reader, error) {
	Cmd := exec.Command(Terminal, TerminalArg, Command)
	Cmd.Env = func() []string {
		StrSlice := make([]string, 0)
		for k, v := range Env {
			StrSlice = append(StrSlice, k+"="+v)
		}
		return StrSlice
	}()
	Stdout := new(bytes.Buffer)
	Stderr := new(bytes.Buffer)
	Cmd.Stdout = Stdout
	Cmd.Stderr = Stderr
	err := Cmd.Run()
	return Stdout, Stderr, err
}
