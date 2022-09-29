package utils

import (
	"bytes"
	"os/exec"
)

//go:generate ../../bin/mockgen -destination mock/mock_command.go -source command.go
// Interface to run commands
type CommandInterface interface {
	Run(string, ...string) (stdout bytes.Buffer, stderr bytes.Buffer, err error)
}

type Command struct {
}

func (c *Command) Run(name string, args ...string) (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	var stdoutbuff, stderrbuff bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdoutbuff
	cmd.Stderr = &stderrbuff

	err = cmd.Run()
	return
}
