package cmd

import (
	"jcron/modules/cron"
	"jcron/modules/handle"
	"testing"
	"time"
)

var testHandle = handle.NewMongoC("php")

// 测试基础运行是否正确
func TestPHP(t *testing.T) {
	channel := make(chan int, 1)
	channel <- 1
	proc := DefaultPHP.Run(testHandle.NewLoger(), channel, "test.php")
	if proc != nil {
		proc.Wait()
	}

	<-time.After(20 * time.Second)
}

// 测试多任务
func TestChannelPHP(t *testing.T) {
	phpjob3 := NewPHPJob(DefaultPHP, testHandle, 2, "delay3.php")

	c := cron.New()
	c.AddJob("test3", "* * * * * ", phpjob3)
	c.Start()
	defer c.Stop()

	<-time.After(20 * time.Second)
}

// 测试中途Kill任务
func TestKillPHP(t *testing.T) {
	killjob := NewPHPJob(DefaultPHP, testHandle, 2, "kill.php")

	c := cron.New()
	c.AddJob("killjob", "* * * * * ", killjob)
	c.Start()
	defer c.Stop()

	<-time.After(10 * time.Second)

	for i, runInfo := range killjob.runInfoList {

		if killjob.Status(i) {
			print(i, " is running, start date is "+runInfo.date+"\n")
		} else {
			print(i, " is not running\n")
		}

		killjob.Kill(i)

		if killjob.Status(i) {
			print(i, " is running, start date is "+runInfo.date+"\n")
		} else {
			print(i, " is not running\n")
		}

		// 每20秒kill一个进程
		<-time.After(20 * time.Second)
	}

	<-time.After(10 * time.Second)
}

// 测试多个计划任务
func TestMultiPHP(t *testing.T) {
	phpjob3 := NewPHPJob(DefaultPHP, testHandle, 2, "delay3.php")
	phpjob6 := NewPHPJob(DefaultPHP, testHandle, 2, "delay6.php")

	c := cron.New()
	c.AddJob("test3second", "* * * * * *", phpjob3)
	c.AddJob("test6second", "* * * * * *", phpjob6)

	c.Start()
	defer c.Stop()

	<-time.After(30 * time.Second)
}

func TestWechatHandler(t *testing.T) {
	var userWechat = handle.NewQyWechat("huali")
	//phpjob3 := NewPHPJob(DefaultPHP, userWechat, 1, "delay3.php")

	// 注意不同的环境要设置不同的缓冲区用于接收日志
	phpjob6 := NewPHPJob(DefaultPHP, userWechat, 3, "delay6.php")

	c := cron.New()
	//c.AddJob("test3second", "* * * * * *", phpjob3)
	c.AddJob("test6second", "* * * * * *", phpjob6)

	c.Start()
	defer c.Stop()

	select {
	case <-time.After(10 * time.Second):
		for i, runInfo := range phpjob6.runInfoList {
			if phpjob6.Status(i) {
				print(i, " is running, start date is "+runInfo.date+"\n")
			} else {
				print(i, " is not running\n")
			}
			print("Kill ", i, "\n")
			phpjob6.Kill(i)
			if phpjob6.Status(i) {
				print(i, " is running, start date is "+runInfo.date+"\n")
			} else {
				print(i, " is not running\n")
			}
		}
	}

	<-time.After(10 * time.Second)
}
