package cmd

import (
	"encoding/json"
	"errors"
	"jcron/modules/cron"
	"jcron/modules/handle"
	"jcron/modules/proc"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// php执行环境
type PHPEnv struct {
	Path string `json: "path"` // php执行文件路径
	Ini  string `json: "ini"`  // php配置文件路径
	Pwd  string `json: "pwd"`  // 工作目录
}

type PHPJob struct {
	env         *PHPEnv
	handler     handle.Handler // 输出处理
	args        []string
	channel     chan int        // 当前任务的执行并发数
	num         int             //最大同时执行的个数
	RunInfoList []*cron.RunInfo // 当前PHPJob正在运行的所有进程句柄
	runLock     sync.Mutex
}

// 检查文件或目录是否存在
// 如果由 filename 指定的文件或目录存在则返回 true，否则返回 false
func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

//var PHPJobList = make(map[string]*PHPJob)

func DefaultPHP(content string) *PHPEnv {
	var env PHPEnv
	err := json.Unmarshal([]byte(content), &env)
	if err != nil {
		log.Printf(content + " is not json format")
	}
	if !Exist(env.Path) {
		log.Printf(env.Path + " not exist")
	}
	if !Exist(env.Ini) {
		log.Printf(env.Ini + " not exist")
	}
	if !Exist(env.Pwd) {
		log.Printf(env.Pwd + " not exist")
	}
	return &env
}

/**
 * 执行PHP
 */
func (env *PHPEnv) Run(loger handle.Loger, job *PHPJob, channel chan int, args ...string) *os.Process {
	// 参数合并，加入配置文件
	iniArgs := []string{"-c", env.Ini}
	args = append(iniArgs, args...)

	cmd := exec.Command(env.Path, args...)

	// 设置工作目录和日志管道
	cmd.Dir = env.Pwd
	cmd.Stdout = loger.NewLogPipe()
	cmd.Stderr = loger.NewErrPipe()

	cmd.Stdout.Write([]byte("start running \n"))
	err := cmd.Start()
	if err != nil {
		// 释放信号量
		<-channel

		cmd.Stderr.Write([]byte(err.Error()))
		data := make(map[string]interface{})
		data["endtime"] = time.Now()
		loger.Update(data)
		return nil
	}

	// 异步等待程序执行完成
	go func() {
		pid := cmd.Process.Pid
		err = cmd.Wait()

		job.runLock.Lock()
		for i, run := range job.RunInfoList {
			if run.Proc.Pid == pid {
				job.RunInfoList = append(job.RunInfoList[:i], job.RunInfoList[i+1:]...)
				break
			}
		}
		job.runLock.Unlock()

		<-channel

		if err != nil {
			cmd.Stderr.Write([]byte(err.Error()))
		} else {
			cmd.Stdout.Write([]byte("finished !\n"))
		}
		data := make(map[string]interface{})
		data["endtime"] = time.Now()
		loger.Update(data)
	}()

	data := make(map[string]interface{})
	data["pid"] = cmd.Process.Pid
	loger.Update(data)
	return cmd.Process
}

/**
 * 创建一个新的PHP任务
 */
func NewPHPJob(phpenv *PHPEnv, handler handle.Handler, num int, args ...string) (*PHPJob, error) {
	if num < 1 {
		return &PHPJob{}, errors.New("Channel must greater than zero")
	}
	return &PHPJob{
		env:         phpenv,
		handler:     handler,
		args:        args,
		channel:     make(chan int, num),
		num:         num,
		RunInfoList: []*cron.RunInfo{},
		runLock:     sync.Mutex{},
	}, nil
}

/**
 * 执行一个PHP任务
 */
func (job *PHPJob) Run(param []string) {
	select {
	case job.channel <- 1:
		loger, objectId := job.handler.NewLoger()
		args := append(job.args, param...)
		proc := job.env.Run(loger, job, job.channel, args...)
		runInfo := &cron.RunInfo{
			time.Now(),
			proc,
			objectId,
		}
		if proc != nil {
			job.runLock.Lock()
			job.RunInfoList = append(job.RunInfoList, runInfo)
			job.runLock.Unlock()

			log.Printf("%s is running, pid is %d\n", job.args[0], proc.Pid)
		}
	default:
		//print(job.args[0] + " channel is full !\n")
	}
}

/**
 * 添加实例
 */
func (job *PHPJob) Add(runInfo *cron.RunInfo) {
	job.runLock.Lock()
	job.RunInfoList = append(job.RunInfoList, runInfo)
	job.runLock.Unlock()
}

// 杀死正在运行的进程
func (job *PHPJob) Kill(objectId string) error {
	job.runLock.Lock()
	for i, runInfo := range job.RunInfoList {
		if runInfo.ObjectId == objectId {
			job.RunInfoList = append(job.RunInfoList[:i], job.RunInfoList[i+1:]...)
			err := proc.KillGroup(runInfo.Proc.Pid)
			job.runLock.Unlock()
			return err
		}
	}
	job.runLock.Unlock()
	return nil
}

/**
 * 获取当前任务的正在运行实例列表
 */
func (job *PHPJob) List() []*cron.RunInfo {
	job.runLock.Lock()
	for i, run := range job.RunInfoList {
		//_, err := os.FindProcess(pid)
		err := proc.Exist(run.Proc.Pid)
		if err != nil {
			job.RunInfoList = append(job.RunInfoList[:i], job.RunInfoList[i+1:]...)
			break
		}
	}
	job.runLock.Unlock()
	return job.RunInfoList
}

//EditJob接口调用
func (job *PHPJob) Channel() int {
	return job.num
}
