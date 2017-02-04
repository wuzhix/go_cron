package web

import (
	"errors"
	"io/ioutil"
	"jcron/modules/cron"
	"jcron/modules/handle"
	"net/http"
	"time"
)

type WebJob struct {
	loger       handle.Handler // 输出处理
	url         string
	channel     chan int
	num         int             //最大同时执行的个数
	RunInfoList []*cron.RunInfo // 当前WebJob正在运行的所有进程句柄
	runLock     chan int
}

/**
 * 创建一个新的PHP任务
 */
func NewWebJob(loger handle.Handler, num int, url string) (*WebJob, error) {
	if num < 1 {
		return &WebJob{}, errors.New("Channel must greater than zero")
	}
	return &WebJob{loger, url, make(chan int, num), num, []*cron.RunInfo{}, make(chan int, 1)}, nil
}

/**
 * 执行一个PHP任务
 */
func (job *WebJob) Run(param []string) {
	select {
	case job.channel <- 1:
		handle, objectId := job.loger.NewLoger()
		logPipe := handle.NewLogPipe()
		errPipe := handle.NewErrPipe()
		logPipe.Write([]byte("start running \n"))
		go func() {
			resp, err := http.Get(job.url)
			job.runLock <- 1
			for i, run := range job.RunInfoList {
				if run.ObjectId == objectId {
					job.RunInfoList = append(job.RunInfoList[:i], job.RunInfoList[i+1:]...)
					break
				}
			}
			<-job.runLock
			<-job.channel
			if err != nil {
				errPipe.Write([]byte(err.Error()))
				data := make(map[string]interface{})
				data["endtime"] = time.Now()
				handle.Update(data)
				return
			}

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				errPipe.Write([]byte(err.Error()))
				data := make(map[string]interface{})
				data["endtime"] = time.Now()
				handle.Update(data)
				return
			}
			logPipe.Write([]byte(body))
			data := make(map[string]interface{})
			data["endtime"] = time.Now()
			handle.Update(data)
		}()
		runInfo := &cron.RunInfo{
			time.Now(),
			nil,
			objectId,
		}
		job.runLock <- 1
		job.RunInfoList = append(job.RunInfoList, runInfo)
		<-job.runLock
	default:
		//print(job.args[0] + " channel is full !\n")
	}
}

/**
 * 添加实例
 */
func (job *WebJob) Add(runInfo *cron.RunInfo) {
	job.runLock <- 1
	job.RunInfoList = append(job.RunInfoList, runInfo)
	<-job.runLock
}

// 杀死正在运行的进程
func (job *WebJob) Kill(objectId string) error {
	for i, runInfo := range job.RunInfoList {
		if runInfo.ObjectId == objectId {
			job.RunInfoList = append(job.RunInfoList[:i], job.RunInfoList[i+1:]...)
			break
		}
	}
	return nil
}

/**
 * 获取当前任务的正在运行实例列表
 */
func (job *WebJob) List() []*cron.RunInfo {
	return job.RunInfoList
}

//EditJob接口调用
func (job *WebJob) Channel() int {
	return job.num
}
