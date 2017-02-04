package main

import (
	"jcron/modules/cron"
	"jcron/modules/handle"
	"jcron/modules/job/cmd"
	"jcron/modules/job/web"

	"errors"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//jsonrpc对象
type Calculator struct{}

func add(name string) error {
	jobData := &cron.JobCollection{}
	find := func(c *mgo.Collection) error {
		return c.Find(bson.M{"name": name, "status": 0}).One(jobData)
	}
	err := handle.WitchCollection(handle.Conf.JobDb, handle.Conf.JobCollection, find)
	if err == nil {
		var jobObj cron.Job
		objHandle := handle.NewMongoC(jobData.Name)
		if jobData.ExecType == "php" {
			jobObj, err = cmd.NewPHPJob(cmd.DefaultPHP(jobData.ExecEnv), objHandle, jobData.Channel, jobData.Content...)
			if err != nil {
				return err
			}
		} else if jobData.ExecType == "http" {
			jobObj, err = web.NewWebJob(objHandle, jobData.Channel, jobData.Content[0])
			if err != nil {
				return err
			}
		} else {
			return errors.New("job not support")
		}
		ret, err := c.AddJob(jobData.Name, jobData.Desc, jobData.Cron, jobObj)
		if err == nil {
			if ret == 0 {
				update := func(c *mgo.Collection) error {
					return c.Update(bson.M{"name": jobData.Name}, bson.M{"$set": bson.M{"status": 1}})
				}
				err = handle.WitchCollection(handle.Conf.JobDb, handle.Conf.JobCollection, update)
				if err != nil {
					return err
				}
			}
		}
		return err
	} else {
		return errors.New("job not exist, or job is running")
	}
}

/**
 * 手动运行一次正在调度的任务
 */
func (t *Calculator) RunOnceJob(testJob *cron.TestJob, reply *int) error {
	for _, entry := range c.Entries() {
		if testJob.Name == entry.Name {
			entry.Job.Run(testJob.Param)
			*reply = 0
			return nil
		}
	}
	*reply = -1
	return errors.New("job is not running")
}

/**
 * jsonrpc接口，启动job
 */
func (t *Calculator) StartJob(name string, reply *int) error {
	log.Printf("StartJob Name : %s\n", name)

	err := add(name)
	if err != nil {
		*reply = -1
		return err
	} else {
		*reply = 0
		return nil
	}
}

/**
 * jsonrpc接口，停止job
 */
func (t *Calculator) StopJob(name string, reply *int) error {
	log.Printf("StopJob Name : %s\n", name)
	c.RemoveFunc(name)
	//更新运行状态
	// 获取正在正常进行调度的任务
	jobData := cron.JobCollection{}
	find := func(c *mgo.Collection) error {
		return c.Find(bson.M{"name": name, "status": 1}).One(&jobData)
	}
	err := handle.WitchCollection(handle.Conf.JobDb, handle.Conf.JobCollection, find)
	if err == nil {
		update := func(c *mgo.Collection) error {
			return c.Update(bson.M{"name": name}, bson.M{"$set": bson.M{"status": 0}})
		}
		handle.WitchCollection(handle.Conf.JobDb, handle.Conf.JobCollection, update)
		*reply = 0
		return nil
	} else {
		*reply = -1
		return errors.New("job not exist, or job is stoped")
	}
}

/**
 * jsonrpc接口，获取job实例
 */
func (t *Calculator) GetJobInstance(name string, reply *[]*cron.RunInfo) error {
	log.Printf("GetJobInstance name : %s\n", name)
	//*reply要赋值，不然接收端会产生invalid error <nil>
	*reply = []*cron.RunInfo{}
	entries := c.Entries()
	if len(entries) > 0 {
		for _, entry := range entries {
			if entry.Name == name {
				*reply = entry.Job.List()
				for _, runInfo := range *reply {
					log.Printf("ObjectId : %s, Date : %s\n", runInfo.ObjectId, runInfo.Date)
				}
			}
		}
	}
	return nil
}

/**
 * jsonrpc接口，杀死job实例
 */
func (t *Calculator) KillJobInstance(jobInstance *cron.JobInstance, reply *int) error {
	log.Printf("KillJobInstance name : %s, objectid : %s\n", jobInstance.JobName, jobInstance.ObjectId)
	entries := c.Entries()
	for _, entry := range entries {
		if entry.Name == jobInstance.JobName {
			err := entry.Job.Kill(jobInstance.ObjectId)
			if err != nil {
				*reply = -1
				return err
			} else {
				*reply = 0
			}
			return err
		}
	}
	return nil
}

func (t *Calculator) GetJobList(flag bool, reply *[]*cron.JobList) error {
	*reply = []*cron.JobList{}
	for _, entry := range c.Entries() {
		*reply = append(*reply, &cron.JobList{entry.Name, entry.Job.List()})
	}
	return nil
}

// 注册RPC服务
func registerRPC() {
	cal := new(Calculator)
	server := rpc.NewServer()
	server.Register(cal)
	server.HandleHTTP(rpc.DefaultRPCPath, rpc.DefaultDebugPath)

	//启动tcp前先延时1s，防止旧程序关闭时端口没有及时释放
	<-time.After(1 * time.Second)
	//启动tcp端口监控
	listener, e := net.Listen("tcp", ":"+handle.Conf.JsonRpcPort)
	if e != nil {
		log.Printf("listen error:", e)
	}

	for {
		//log.Printf("start listen\n")
		if conn, err := listener.Accept(); err != nil {
			log.Printf("accept error: " + err.Error())
		} else {
			//log.Printf("new connection established\n")
			//异步处理rpc请求
			go server.ServeCodec(jsonrpc.NewServerCodec(conn))
		}
	}
}
