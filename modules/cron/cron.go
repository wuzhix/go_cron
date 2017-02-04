// This library implements a cron spec parser and runner.  See the README for
// more details.
package cron

import (
	"log"
	"os"
	"runtime"
	"sort"
	"time"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the schedule. It may be started, stopped, and the entries may
// be inspected while running.
type Cron struct {
	entries  []*Entry
	stop     chan struct{} // 停止任务
	add      chan *Entry   // 添加任务
	remove   chan string   // 删除任务
	snapshot chan []*Entry
	running  bool
	ErrorLog *log.Logger
	location *time.Location
}

type RunInfo struct {
	Date     time.Time   //启动时间
	Proc     *os.Process // 命令行进程句柄
	ObjectId string
}

type JobList struct {
	Name        string
	RunInstance []*RunInfo
}

type TestJob struct {
	Name  string
	Param []string
}

// Job is an interface for submitted cron jobs.
type Job interface {
	Run(param []string)
	Add(runInfo *RunInfo)
	Kill(objectId string) error
	List() []*RunInfo
	Channel() int
}

//job集合
type JobCollection struct {
	//job名称，不能重复
	Name string
	//类别id
	Category string
	//描述
	Desc string
	//执行频率
	Cron string
	//最大并发数
	Channel int
	//运行内容
	Content []string
	//运行状态，0未运行，1运行中
	Status int
	//添加人
	AddPerson string
	//添加时间戳
	AddTime string
	//可见人
	ViewPerson string
	//通知人
	NoticePerson string
	//最后编辑人
	EditPerson string
	//最后编辑时间
	EditTime string
	//执行程序环境名称
	Env string
	//执行程序类型
	ExecType string
	//执行程序环境变量
	ExecEnv string
}

//job实例
type JobInstance struct {
	//job name
	JobName string
	//objectId
	ObjectId string
}

// The Schedule describes a job's duty cycle.
type Schedule interface {
	// Return the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

// Entry consists of a schedule and the func to execute on that schedule.
type Entry struct {
	//任务名称，唯一识别标识
	Name string
	// 任务描述信息
	Desc string

	// 执行频率数值
	Cron string

	// The schedule on which this job should be run.
	Schedule Schedule

	// The next time the job will run. This is the zero time if Cron has not been
	// started or this entry's schedule is unsatisfiable
	Next time.Time

	// The last time this job was run. This is the zero time if the job has never
	// been run.
	Prev time.Time

	// The Job to run.
	Job Job
}

// byTime is a wrapper for sorting the entry array by time
// (with zero time at the end).
type byTime []*Entry

func (s byTime) Len() int      { return len(s) }
func (s byTime) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byTime) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	if s[i].Next.IsZero() {
		return false
	}
	if s[j].Next.IsZero() {
		return true
	}
	return s[i].Next.Before(s[j].Next)
}

// New returns a new Cron job runner, in the Local time zone.
func New() *Cron {
	return NewWithLocation(time.Now().Location())
}

// NewWithLocation returns a new Cron job runner.
func NewWithLocation(location *time.Location) *Cron {
	return &Cron{
		entries:  nil,
		add:      make(chan *Entry),
		remove:   make(chan string),
		stop:     make(chan struct{}),
		snapshot: make(chan []*Entry),
		running:  false,
		ErrorLog: nil,
		location: location,
	}
}

// A wrapper that turns a func() into a cron.Job
type FuncJob func()

func (f FuncJob) Run(param []string)         { f() }
func (f FuncJob) Add(runInfo *RunInfo)       { f() }
func (f FuncJob) Kill(objectId string) error { return nil }
func (f FuncJob) List() []*RunInfo           { return []*RunInfo{} }
func (f FuncJob) Channel() int               { return 0 }

// AddFunc adds a func to the Cron to be run on the given schedule.
func (c *Cron) AddFunc(name, desc, cron string, cmd func()) (int, error) {
	return c.AddJob(name, desc, cron, FuncJob(cmd))
}

// 删除计划任务
func (c *Cron) RemoveFunc(name string) {
	if !c.running {
		c.delEntry(name)
	} else {
		c.remove <- name
	}
}

// AddJob adds a Job to the Cron to be run on the given schedule.
func (c *Cron) AddJob(name, desc, cron string, cmd Job) (int, error) {
	schedule, err := Parse(cron)
	if err != nil {
		return -1, err
	}
	scheduleId := c.Schedule(name, desc, cron, schedule, cmd)
	return scheduleId, nil
}

// Schedule adds a Job to the Cron to be run on the given schedule.
func (c *Cron) Schedule(name, desc, cron string, schedule Schedule, cmd Job) int {
	//这里判断一下，添加重复任务时（name重复），id返回0
	for _, entry := range c.entries {
		if entry.Name == name {
			return -1
		}
	}
	var entry = &Entry{
		Name:     name,
		Desc:     desc,
		Cron:     cron,
		Schedule: schedule,
		Job:      cmd,
	}
	if !c.running {
		c.entries = append(c.entries, entry)
	} else {
		c.add <- entry
	}

	return 0
}

// Entries returns a snapshot of the cron entries.
func (c *Cron) Entries() []*Entry {
	if c.running {
		c.snapshot <- nil
		x := <-c.snapshot
		return x
	}
	return c.entrySnapshot()
}

// Location gets the time zone location
func (c *Cron) Location() *time.Location {
	return c.location
}

// Start the cron scheduler in its own go-routine.
func (c *Cron) Start() {
	if c.running {
		return
	}
	c.running = true
	go c.run()
}

func (c *Cron) runWithRecovery(j Job) {
	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			c.logf("cron: panic running job: %v\n%s", r, buf)
		}
	}()
	j.Run([]string{})
}

// Run the scheduler.. this is private just due to the need to synchronize
// access to the 'running' state variable.
func (c *Cron) run() {
	// Figure out the next activation times for each entry.
	now := time.Now().In(c.location)
	for _, entry := range c.entries {
		entry.Next = entry.Schedule.Next(now)
	}

	for {
		// Determine the next entry to run.
		sort.Sort(byTime(c.entries))

		var effective time.Time
		if len(c.entries) == 0 || c.entries[0].Next.IsZero() {
			// If there are no entries yet, just sleep - it still handles new entries
			// and stop requests.
			effective = now.AddDate(10, 0, 0)
		} else {
			effective = c.entries[0].Next
		}

		timer := time.NewTimer(effective.Sub(now))
		select {
		case now = <-timer.C:
			now = now.In(c.location)
			// Run every entry whose next time was this effective time.
			for _, e := range c.entries {
				if e.Next != effective {
					break
				}
				go c.runWithRecovery(e.Job)
				e.Prev = e.Next
				e.Next = e.Schedule.Next(now)
			}
			continue

		case newEntry := <-c.add:
			c.entries = append(c.entries, newEntry)
			newEntry.Next = newEntry.Schedule.Next(time.Now().In(c.location))

		case name := <-c.remove:
			c.delEntry(name)

		case <-c.snapshot:
			c.snapshot <- c.entrySnapshot()

		case <-c.stop:
			timer.Stop()
			return
		}

		// 'now' should be updated after newEntry and snapshot cases.
		now = time.Now().In(c.location)
		timer.Stop()
	}
}

// Logs an error to stderr or to the configured error log
func (c *Cron) logf(format string, args ...interface{}) {
	if c.ErrorLog != nil {
		c.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// 删除计划任务
func (c *Cron) delEntry(name string) {
	for i, entry := range c.entries {
		if entry.Name == name {
			c.entries = append(c.entries[:i], c.entries[i+1:]...)
		}
	}
}

// Stop stops the cron scheduler if it is running; otherwise it does nothing.
func (c *Cron) Stop() {
	if !c.running {
		return
	}
	c.stop <- struct{}{}
	c.running = false
}

// entrySnapshot returns a copy of the current cron entry list.
func (c *Cron) entrySnapshot() []*Entry {
	entries := []*Entry{}
	for _, e := range c.entries {
		entries = append(entries, &Entry{
			Name:     e.Name,
			Desc:     e.Desc,
			Cron:     e.Cron,
			Schedule: e.Schedule,
			Next:     e.Next,
			Prev:     e.Prev,
			Job:      e.Job,
		})
	}
	return entries
}
