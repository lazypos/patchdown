package patchdown

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	logname = "run.log"
)

type STLogCollect struct {
	HandleLog *os.File
	MuxLog    sync.Mutex
	LogName   string
}

var GLogCollect = &STLogCollect{}

func (this *STLogCollect) Start() error {
	this.LogName = filepath.Join(GExePath, logname)

	finfo, err := os.Stat(this.LogName)
	if err == nil && finfo.Size() > (1024*1024*100) {
		os.Remove(this.LogName)
		log.Println("日志过大，删除日志")
	}
	this.HandleLog, err = os.OpenFile(this.LogName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0x666)
	if err != nil {
		log.Println("无法打开日志文件.", err)
		return err
	}
	return nil
}

func (this *STLogCollect) ToRunLog(logs string) {
	this.MuxLog.Lock()
	defer this.MuxLog.Unlock()
	//log.Println(logs)
	this.HandleLog.WriteString(time.Now().String()[:19] + " " + logs)
}

func (this *STLogCollect) ZipRunLog() (string, error) {
	this.MuxLog.Lock()
	defer this.MuxLog.Unlock()

	this.HandleLog.Close()
	err := CompressFile2Zip(this.LogName, this.LogName+".zip")
	if err != nil {
		log.Println("压缩失败", err)
		return "", err
	}

	this.HandleLog, err = os.OpenFile(this.LogName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0x666)
	if err != nil {
		log.Println("打开日志文件失败.", err)
		return "", err
	}
	return this.LogName + ".zip", nil
}
