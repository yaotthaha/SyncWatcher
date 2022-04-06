package main

import (
	"bytes"
	"io"
	"os/exec"
)

func CommandRun(Command string, Terminal, TerminalArg string) (io.Reader, io.Reader, error) {
	Cmd := exec.Command(Terminal, TerminalArg, Command)
	Stdout := new(bytes.Buffer)
	Stderr := new(bytes.Buffer)
	Cmd.Stdout = Stdout
	Cmd.Stderr = Stderr
	err := Cmd.Run()
	return Stdout, Stderr, err
}
