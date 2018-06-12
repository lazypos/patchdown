package patchdown

import (
	"encoding/xml"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type STStatMgr struct {
	DownCounts int
	DownNow    int
	XMLDate    string
	CRC32      uint32
	MuxStat    sync.Mutex
}

var GSTStatMgr = &STStatMgr{}

func (this *STStatMgr) Init() {
	GLogCollect.ToRunLog(fmt.Sprintln(`初始化STStatMgr...`))
	defer GLogCollect.ToRunLog(fmt.Sprintln(`初始化STStatMgr完毕...`))
	this.LoadXMLZipDate()
	go this.Routine_work()
}

func (this *STStatMgr) Routine_work() {
	ticker := time.NewTicker(time.Minute)
	for {
		select {
		case <-ticker.C:
			this.LoadXMLZipDate()
		}
	}
}

func (this *STStatMgr) LoadXMLDate() {
	this.MuxStat.Lock()
	defer this.MuxStat.Unlock()

	type STDate struct {
		Date string `xml:"global>date"`
	}

	xmlPath := GExePath + "/patchxml.xml"
	xmlText, err := ioutil.ReadFile(xmlPath)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln(`读取索引文件错误`, err))
		return
	}

	st := &STDate{}
	if err = xml.Unmarshal([]byte(xmlText), st); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln(`解析索引文件错误`, err))
		return
	}
	this.XMLDate = st.Date
}

//获取索引压缩包的更新时间
func (this *STStatMgr) LoadXMLZipDate() {

	this.MuxStat.Lock()
	defer this.MuxStat.Unlock()

	zipPath := filepath.Join(GExePath, "/patchxml.zip")

	//獲取文件的最後更新時間
	finfo, err := os.Stat(zipPath)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln(`获取索引压缩文件信息失败`, err))
		return
	}
	if finfo.ModTime().String()[:19] != this.XMLDate {
		this.XMLDate = finfo.ModTime().String()[:19]

		////獲取補丁索引的CRC
		ziptext, err := ioutil.ReadFile(zipPath)
		if err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln(`读取索引压缩文件失败`, err))
			return
		}
		this.CRC32 = crc32.ChecksumIEEE(ziptext)
		GLogCollect.ToRunLog(fmt.Sprintln(`索引CRC=`, this.CRC32))
	}
}

func (this *STStatMgr) GetXmlDate() string {
	this.MuxStat.Lock()
	defer this.MuxStat.Unlock()

	return this.XMLDate
}

func (this *STStatMgr) GetXmlCrc() uint32 {
	this.MuxStat.Lock()
	defer this.MuxStat.Unlock()

	return this.CRC32
}
