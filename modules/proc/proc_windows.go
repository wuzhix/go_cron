package proc

import (
	"os"
)

func kill(pid int) error {
	process, err := os.FindProcess(pid)
	if err == nil {
		return process.Kill()
	}
	return nil
}

func killGroup(pid int) error {
	process, err := os.FindProcess(pid)
	if err == nil {
		return process.Kill()
	} else {
		return err
	}
}

func exist(pid int) error {
	_, err := os.FindProcess(pid)
	return err
}
