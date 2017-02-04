package handle

import (
	"io"
	"os"
)

type console struct {
	io.Writer
}

var Console = &console{
	io.Writer(os.Stdout),
}

func (c *console) NewLoger() (Loger, string) {
	return c, ""
}

func (c *console) NewLogPipe() io.Writer {
	return c
}

func (c *console) NewErrPipe() io.Writer {
	return c
}

func (c console) Update(data map[string]interface{}) {

}
