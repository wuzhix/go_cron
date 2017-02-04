package proc

// 杀掉进程
func Kill(pid int) error {
	return kill(pid)
}

// 杀死进程组
func KillGroup(pid int) error {
	return killGroup(pid)
}

// 查看进程是否存在
func Exist(pid int) error {
	return exist(pid)
}
