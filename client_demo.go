// client.go
package main

import (
	"fmt"
	"jcron/modules/cron"
	"log"
	"net"
	"net/rpc/jsonrpc"
	"time"
)

func main() {

	client, err := net.Dial("tcp", "127.0.0.1:1234")
	if err != nil {
		log.Printf("dialing:", err)
	}
	// Synchronous call
	var reply = 0
	name1 := "php1"
	//name2 := "http1"
	c := jsonrpc.NewClient(client)

	//获取任务实例列表
	list := []*cron.RunInfo{}
	err = c.Call("Calculator.GetJobInstance", name1, &list)
	if err != nil {
		log.Printf("arith error:", err)
	}
	fmt.Printf("GetJobInstance %s\n", name1)
	for _, runInfo := range list {
		fmt.Printf("Date : %s, ObjectId : %s\n", runInfo.Date, runInfo.ObjectId)
	}

	//停止任务
	<-time.After(10 * time.Second)
	fmt.Printf("stop job : %s\n", name1)
	reply = 0
	err = c.Call("Calculator.StopJob", name1, &reply)
	if err != nil {
		log.Printf("arith error:", err)
	}

	//获取任务实例列表
	<-time.After(10 * time.Second)
	list = []*cron.RunInfo{}
	err = c.Call("Calculator.GetJobInstance", name1, &list)
	if err != nil {
		log.Printf("arith error:", err)
	}
	fmt.Printf("GetJobInstance %s\n", name1)
	for _, runInfo := range list {
		fmt.Printf("Date : %s, ObjectId : %s\n", runInfo.Date, runInfo.ObjectId)
	}

	//启动任务，重新生成id
	<-time.After(10 * time.Second)
	reply = 0
	err = c.Call("Calculator.StartJob", name1, &reply)
	if err != nil {
		log.Printf("arith error:", err)
	}
	fmt.Printf("start job : %s\n", name1)

	//获取任务实例列表
	<-time.After(10 * time.Second)
	list = []*cron.RunInfo{}
	err = c.Call("Calculator.GetJobInstance", name1, &list)
	if err != nil {
		log.Printf("arith error:", err)
	}
	fmt.Printf("GetJobInstance %s\n", name1)
	for pid, runInfo := range list {
		fmt.Printf("Date : %s, ObjectId : %s\n", runInfo.Date, runInfo.ObjectId)
		fmt.Printf("KillJobInstance %d\n", pid)
		err = c.Call("Calculator.KillJobInstance", &cron.JobInstance{name1, runInfo.ObjectId}, &reply)
		if err != nil {
			log.Printf("arith error:", err)
		}
	}

	//获取任务实例列表
	<-time.After(10 * time.Second)
	list = []*cron.RunInfo{}
	err = c.Call("Calculator.GetJobInstance", name1, &list)
	if err != nil {
		log.Printf("arith error:", err)
	}
	fmt.Printf("GetJobInstance %s\n", name1)
	for _, runInfo := range list {
		fmt.Printf("Date : %s, ObjectId : %s\n", runInfo.Date, runInfo.ObjectId)
	}

	//获取任务列表
	<-time.After(10 * time.Second)
	fmt.Printf("GetJobList \n")
	var flag = true
	jobList := []*cron.JobList{}
	err = c.Call("Calculator.GetJobList", flag, &jobList)
	if err != nil {
		log.Printf("arith error:", err)
	}
	for _, job := range jobList {
		log.Printf("job name : %s, job count : %d", job.Name, len(job.RunInstance))
	}

	<-time.After(10 * time.Second)
}
