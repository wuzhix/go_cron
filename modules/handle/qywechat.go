package handle

import (
	"bytes"
	"io"
	"juanpi_modules/qywechat"
)

// 企业微信作为日志处理工具
type QyWechat struct {
	*qywechat.App
	recevers string
}

type WechatLoger struct {
	buff   *bytes.Buffer
	wechat *QyWechat
}

var monitorApp = qywechat.NewApp(34)

func NewQyWechat(recevers string) Handler {
	return &QyWechat{
		monitorApp,
		recevers,
	}
}

func (wechat *QyWechat) NewLoger() (Loger, string) {
	var buff []byte
	return &WechatLoger{
		bytes.NewBuffer(buff),
		wechat,
	}, ""
}

func (log *WechatLoger) NewLogPipe() io.Writer {
	return log.buff
}

func (log *WechatLoger) NewErrPipe() io.Writer {
	return log.buff
}

// 日志保存
func (log *WechatLoger) Save() {
	go log.wechat.SendText(log.wechat.recevers, log.buff.String())
	log = nil
}

func (log *WechatLoger) Update(data map[string]interface{}) {

}
