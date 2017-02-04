// server.go
/**
 1、StopJob接口会将job从job列表移除，此时job的并发数无法控制。调用StartJob会重新添加job，
	也会重新控制并发数，此时job的并发数会大于设定值，直到StopJob停止的job实例运行完才会恢复正常
 2、加载job和快照时，因为无法添加不运行的job。如果先加载快照，会运行job，然后再加载快照实例，这时再加载job，又会运行job。
	目前流程为先加载job，这时运行job，然后再加载快照实例，此时job并发数会大于设定值，直到快照实例运行完才会恢复正常
*/
package main

import (
	"encoding/json"
	"jcron/modules/cron"
	"jcron/modules/handle"
	"jcron/modules/job/cmd"
	"jcron/modules/job/web"
	"jcron/modules/proc"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//定时任务对象
var c = cron.New()

//job实例快照
type JobInstanceSnapshot struct {
	//进程id
	Pid int
	//进程启动时间
	Date time.Time
}

//jobSnapshot集合
type JobSnapshot struct {
	//job名称，不能重复
	Name string
	//job实例集
	Instance []JobInstanceSnapshot
}

//curProcess集合
type CurProcess struct {
	//进程id
	Pid  int
	Date time.Time
}

/**
 * 保存job快照
 */
func SaveJobSnapshot() {
	log.Printf("SaveJobSnapshot\n")

	// 获取当前正在调度的计划任务
	entries := c.Entries()
	if len(entries) > 0 {
		//保存快照前先清理已有数据，保证每次读取的都是最近一次的有效快照
		remove := func(c *mgo.Collection) error {
			_, err := c.RemoveAll(nil)
			return err
		}
		err := handle.WitchCollection(handle.Conf.JobDb, handle.Conf.JobSnapshotCollection, remove)

		for _, entry := range entries {
			jobSnapshot := &JobSnapshot{}
			jobSnapshot.Name = entry.Name

			// 获取每一个正在运行的计划任务的实例
			for _, instance := range entry.Job.List() {
				//有进程id的实例保存到快照，http请求因为找不到进程id，所以不保存到快照
				if instance.Proc != nil {
					err := proc.Exist(instance.Proc.Pid)
					if err == nil {
						//正在运行的实例加入到快照中
						jobSnapshot.Instance = append(jobSnapshot.Instance, JobInstanceSnapshot{instance.Proc.Pid, instance.Date})
						log.Printf("SaveJobSnapshot Name : %s, ObjectId : %s, Date : %s\n", jobSnapshot.Name, instance.ObjectId, instance.Date)
					}
				}
			}

			// 将快照插入mongo
			insert := func(c *mgo.Collection) error {
				return c.Insert(jobSnapshot)
			}
			err = handle.WitchCollection(handle.Conf.JobDb, handle.Conf.JobSnapshotCollection, insert)
			if err != nil {
				log.Fatal("SaveJobSnapshot Insert error:", err)
			}
		}
	}
}

/**
 * 加载job和job快照
 */
func LoadJobAndSnapshot() {
	log.Printf("LoadJobAndSnapshot\n")

	//加载job
	var jobList []cron.JobCollection
	// 获取正在正常进行调度的任务
	find := func(c *mgo.Collection) error {
		return c.Find(bson.M{"status": 1}).All(&jobList)
	}
	err := handle.WitchCollection(handle.Conf.JobDb, handle.Conf.JobCollection, find)
	if err != nil {
		log.Fatal("Load job Find error:", err)
	}
	var jobObj cron.Job
	for _, jobData := range jobList {
		objHandle := handle.NewMongoC(jobData.Name)
		if jobData.ExecType == "php" {
			jobObj, err = cmd.NewPHPJob(cmd.DefaultPHP(jobData.ExecEnv), objHandle, jobData.Channel, jobData.Content...)
			if err != nil {
				log.Printf("Load job %s error", jobData.Name)
			}
		} else if jobData.ExecType == "http" {
			jobObj, err = web.NewWebJob(objHandle, jobData.Channel, jobData.Content[0])
			if err != nil {
				log.Printf("Load job %s error", jobData.Name)
			}
		}
		_, err := c.AddJob(jobData.Name, jobData.Desc, jobData.Cron, jobObj)
		if err != nil {
			log.Printf("AddJob %s error", jobData.Name)
		}
	}

	//延时一秒读取， 上一个进程关闭调用SaveJobSnapshot需要时间
	<-time.After(1 * time.Second)

	//加载jobSnapshot，并把之前正在运行的实例加入当对应的计划任务中进行管理
	var jobSnapshotList []JobSnapshot
	find = func(c *mgo.Collection) error {
		return c.Find(nil).All(&jobSnapshotList)
	}
	err = handle.WitchCollection(handle.Conf.JobDb, handle.Conf.JobSnapshotCollection, find)
	if err != nil {
		log.Fatal("Load jobSnapshot Find error:", err)
	}

	// 加载当前调度任务快照
	entries := c.Entries()
	for _, jobSnapshot := range jobSnapshotList {
		for _, jobInstanceSnapshot := range jobSnapshot.Instance {
			for _, entry := range entries {
				if jobSnapshot.Name == entry.Name {
					//_, err := os.FindProcess(jobInstanceSnapshot.Pid)
					err := proc.Exist(jobInstanceSnapshot.Pid)
					if err == nil {
						// 获取当前正在执行的任务进程句柄
						process, _ := os.FindProcess(jobInstanceSnapshot.Pid)
						runInfo := &cron.RunInfo{
							Date: jobInstanceSnapshot.Date,
							Proc: process,
						}

						//加载成功后，正在运行的实例加到job的实例列表
						entry.Job.Add(runInfo)
					}
				}
			}
		}
	}

	//输出日志
	for _, entry := range entries {
		runInfo := entry.Job.List()
		for pid, run := range runInfo {
			log.Printf("LoadJobAndSnapshot Name : %s, Pid : %d, Date : %s\n", entry.Name, pid, run.Date)
		}
	}
}

/**
 * 初始化
 */
func init() {
	// 获取当前程序的pid
	curPid := os.Getpid()
	log.Printf("Init cur Process %d \n", curPid)

	//加载配置文件
	file, _ := os.Open("conf.json")
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&handle.Conf)
	if err != nil {
		log.Fatal("load conf.json error:", err)
	}

	//关闭正在执行的进程
	var cur CurProcess
	find := func(c *mgo.Collection) error {
		return c.Find(nil).One(&cur)
	}
	err = handle.WitchCollection(handle.Conf.JobDb, handle.Conf.CurProcessCollection, find)

	pid := cur.Pid
	if pid > 0 {
		err := proc.Exist(pid)
		if err == nil {
			proc.Kill(pid)
		}

		//将当前进程id更新到数据库
		update := func(c *mgo.Collection) error {
			return c.Update(bson.M{"pid": pid}, bson.M{"$set": bson.M{"pid": curPid, "date": time.Now()}})
		}
		err = handle.WitchCollection(handle.Conf.JobDb, handle.Conf.CurProcessCollection, update)
		if err != nil {
			panic(err)
		}

	} else {
		//第一次启动时将当前进程id写入数据库
		insert := func(c *mgo.Collection) error {
			return c.Insert(&CurProcess{curPid, time.Now()})
		}
		err = handle.WitchCollection(handle.Conf.JobDb, handle.Conf.CurProcessCollection, insert)
	}

	//加载任务和快照
	LoadJobAndSnapshot()
}

/**
 * 进程接收到SIGTERM信号时，先停止任务，保存job快照，再退出当前进程
 */
func HookSignal() {
	log.Printf("HookSignal\n")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		//等待SIGTERM关闭信号
		<-sigs
		//1、停止计划任务
		c.Stop()
		//2、保存job快照
		SaveJobSnapshot()
		//3、退出当前进程
		os.Exit(1)
	}()
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	HookSignal()
	log.Printf("StartServer\n")
	c.Start()
	registerRPC()
}
