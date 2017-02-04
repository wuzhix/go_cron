package proc

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func kill(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}

func killGroup(pid int) error {
	println("killGroup", pid)
	root := "/proc"
	files, _ := ioutil.ReadDir(root)
	for _, f := range files {
		if f.IsDir() {
			curpid, err1 := strconv.Atoi(f.Name())
			if err1 != nil {
				println(err1.Error())
			} else {
				fp, err2 := os.Open(root + "/" + f.Name() + "/stat")
				if err2 != nil {
					println(err2.Error())
				} else {
					defer fp.Close()
					buff := make([]byte, 1024)
					fp.Read(buff)
					ret := strings.Split(string(buff), " ")
					ppid, err3 := strconv.Atoi(ret[3])
					if err3 != nil {
						println(err3.Error())
					} else {
						if ppid == pid {
							println(curpid, "parent pid is", ppid)
							syscall.Kill(curpid, syscall.SIGKILL)
						}
					}
				}
			}
		}
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

func exist(pid int) error {
	return syscall.Kill(pid, 0)
}
