// 数据库设计
// 一个计划任务一个collection,一次执行一个document
package handle

import (
	"fmt"
	"io"
	"jcron/modules/cron"
	"juanpi_modules/qywechat"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Configuration struct {
	JobDb                 string
	JobLogDb              string
	DbHost                string
	DbPort                string
	JobCollection         string
	JobSnapshotCollection string
	CurProcessCollection  string
	ErrLogCollection      string
	ErrLogViewCollection  string
	OperateLogCollection  string
	PhpBinPath            string
	PhpIniPath            string
	JobPath               string
	JsonRpcPort           string
}

var Conf = Configuration{}

type logItem struct {
	Time     time.Time
	FromType int
	Content  string
}

type logList []logItem

type pipe struct {
	record     *Record
	collection string
}

type logPipe pipe
type errPipe pipe

type mongoC string

type MongoLog struct {
	logPipe    logPipe // 运行日志管道
	errPipe    errPipe // 错误日志管道
	collection string  // 数据表
}

type Record struct {
	Id        bson.ObjectId `bson:"_id"`
	Name      string        // 任务名称
	StartTime time.Time     // 开始时间
	EndTime   time.Time     // 结束时间
	Content   logList       // 日志内容
	Pid       int           // 实例进程id
	Result    int           // 运行结果，1正常，2异常
}

type ErrLog struct {
	Name  string
	Time  time.Time
	LogId string
}

var mgoSession *mgo.Session

func getSession() *mgo.Session {
	if mgoSession == nil {
		var err error
		mgoSession, err = mgo.Dial(Conf.DbHost + ":" + Conf.DbPort)
		if err != nil {
			panic(err) //直接终止程序运行
		}
	}
	//最大连接池默认为4096
	return mgoSession.Clone()
}

//公共方法，获取collection对象
func WitchCollection(database, collection string, s func(*mgo.Collection) error) error {
	session := getSession()
	defer session.Close()
	c := session.DB(database).C(collection)
	return s(c)
}

func NewMongoC(job string) Handler {
	return mongoC(job)
}

func (c mongoC) NewLoger() (Loger, string) {
	var log logList
	objectId := bson.NewObjectId()
	nowTime := time.Now()
	record := &Record{
		objectId,
		string(c),
		nowTime,
		nowTime,
		log,
		0,
		1,
	}
	insert := func(c *mgo.Collection) error {
		return c.Insert(record)
	}
	WitchCollection(Conf.JobLogDb, string(c), insert)

	return &MongoLog{
		logPipe{record, string(c)},
		errPipe{record, string(c)},
		string(c),
	}, fmt.Sprintf(`%x`, string(objectId))
}

// 正常日志管道
func (l *logPipe) Write(p []byte) (n int, err error) {
	nowTime := time.Now()
	item := logItem{
		nowTime,
		0,
		string(p),
	}
	update := func(c *mgo.Collection) error {
		return c.Update(bson.M{"_id": l.record.Id}, bson.M{"$push": bson.M{"content": &item}})
	}
	WitchCollection(Conf.JobLogDb, l.collection, update)

	return len(p), nil
}

// 错误日志管道
func (e *errPipe) Write(p []byte) (n int, err error) {
	curTime := time.Now()
	item := logItem{
		curTime,
		1,
		string(p),
	}
	update := func(c *mgo.Collection) error {
		return c.Update(bson.M{"_id": e.record.Id}, bson.M{"$push": bson.M{"content": &item}})
	}
	WitchCollection(Conf.JobLogDb, e.collection, update)

	// 写入错误告警日志
	errLog := &ErrLog{e.record.Name, curTime, fmt.Sprintf(`%x`, string(e.record.Id))}
	upsert := func(c *mgo.Collection) error {
		_, err := c.Upsert(bson.M{"logid": errLog.LogId}, &errLog)
		return err
	}
	WitchCollection(Conf.JobDb, Conf.ErrLogCollection, upsert)

	//微信报警
	job := cron.JobCollection{}
	find := func(c *mgo.Collection) error {
		return c.Find(bson.M{"name": e.collection}).One(&job)
	}
	err = WitchCollection(Conf.JobDb, Conf.JobCollection, find)
	qywechat.Alert(job.NoticePerson, job.Name, "任务名称："+job.Name+"\n告警内容："+string(p))

	return len(p), nil
}

// 运行日志管道
func (m *MongoLog) NewLogPipe() io.Writer {
	return &m.logPipe
}

// 错误日志管道
func (m *MongoLog) NewErrPipe() io.Writer {
	return &m.errPipe
}

func (m *MongoLog) Update(data map[string]interface{}) {
	update := func(c *mgo.Collection) error {
		return c.Update(bson.M{"_id": m.logPipe.record.Id}, bson.M{"$set": &data})
	}
	WitchCollection(Conf.JobLogDb, m.collection, update)
}
