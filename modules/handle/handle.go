package handle

import (
	"io"
)

// 日志相关接口
type Loger interface {
	NewLogPipe() io.Writer
	NewErrPipe() io.Writer
	Update(data map[string]interface{})
}

// 新建日志接口
type Handler interface {
	NewLoger() (Loger, string)
}
