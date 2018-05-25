package patchdown

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	THREAD_COUNTS          = 5
	SQL_LOAD_DOWNPATCHS    = `select ifnull(URL,'') from Dict_PatchFile where ifnull(IsExist,0)=0 and  ifnull(Disabled,0)=0 and ifnull(DownTimes,0)<=3 limit 500`
	SQL_LOAD_REDOWNPATCHS  = `select ifnull(URL,'') from Dict_PatchFile where ifnull(IsExist,0)=1 and ifnull(Disabled,0)=0 and ifnull(DownTimes,0)<=3 and ifnull(DownTimes,0)<>0 limit 50`
	SQL_UPDATE_FILESTAT_OK = `update Dict_PatchFile set IsExist=1, DownTimes=%v, DownloadTime='%v',CheckSum='%v' where url='%v'`
	SQL_UPDATE_FILESTAT_NO = `update Dict_PatchFile set IsExist=0, DownTimes=DownTimes+1 where url='%v'`
)

type STPatchDown struct {
	chQueue chan string
}

var GSTPatchDown = &STPatchDown{}

func (this *STPatchDown) Start() {
	this.chQueue = make(chan string, 1000)
	go this.RoutineLoad()
	w := &sync.WaitGroup{}
	w.Add(THREAD_COUNTS)
	for i := 0; i < THREAD_COUNTS; i++ {
		go this.RoutineDown(w)
	}
	w.Wait()
}

//从数据库加载需要下载的补丁URL
func (this *STPatchDown) LoadURLList(sql string) {
	rows, err := GSqlOpt.Query(sql)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("加载下载补丁列表错误", err, sql))
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var url string
		if err = rows.Scan(&url); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("scan获取下载URL错误", err))
			return
		}
		count++
		this.chQueue <- url
	}
	GLogCollect.ToRunLog(fmt.Sprintln("加载下载补丁列表成功", count, "条"))
}

//加载待下载补丁列表
func (this *STPatchDown) RoutineLoad() {
	for {
		if len(this.chQueue) < 10 {
			//先加载未下载的
			this.LoadURLList(SQL_LOAD_DOWNPATCHS)
			//再加载已下载的
			this.LoadURLList(SQL_LOAD_REDOWNPATCHS)
		}
		time.Sleep(time.Second * 30)
	}
}

//下载补丁线程
func (this *STPatchDown) RoutineDown(w *sync.WaitGroup) {
	defer w.Done()

	var url string = ""
	for {
		//获取一个待下载补丁
		url = ""
		select {
		case url = <-this.chQueue:
		}
		if len(url) != 0 {
			GLogCollect.ToRunLog(fmt.Sprintln("开始下载补丁", url))
			idx := strings.LastIndex(url, `/`)
			if idx == -1 {
				GLogCollect.ToRunLog(fmt.Sprintln("下载路径错误", url))
				continue
			}
			patchDir := GetPatchStoreDir()
			f, err := os.Stat(patchDir)
			if err != nil || !f.IsDir() {
				GLogCollect.ToRunLog(fmt.Sprintln("补丁下载目录错误", patchDir))
				continue
			}
			filename := filepath.Join(patchDir, url[idx+1:])
			filename = strings.Replace(filename, "-", "_", -1)

			this.DownLoadPatch(url, filename)
			GLogCollect.ToRunLog(fmt.Sprintln("补丁下载完成, 计算补丁CRC"))
			shaVal, ok := this.CheckPatchEx(filename)
			//下载失败
			if len(shaVal) == 0 {
				GLogCollect.ToRunLog(fmt.Sprintln("补丁处理失败"))
				GSqlOpt.Execute(fmt.Sprintf(SQL_UPDATE_FILESTAT_NO, url))
			} else {
				//根据校验是否一致做不同入库处理
				if ok {
					sql := fmt.Sprintf(SQL_UPDATE_FILESTAT_OK, 0, time.Now().String()[:19], shaVal, url)
					GLogCollect.ToRunLog(fmt.Sprintln("补丁处理完成", sql))
					GSqlOpt.Execute(sql)
				} else {
					sql := fmt.Sprintf(SQL_UPDATE_FILESTAT_OK, "[DownTimes]+1", shaVal, url)
					GLogCollect.ToRunLog(fmt.Sprintln("补丁处理完成，校验不一致", shaVal, sql))
					GSqlOpt.Execute(sql)
				}
			}
		}
		time.Sleep(time.Second)
	}
}

//下载补丁
func (this *STPatchDown) DownLoadPatch(url, filename string) {
	//下载补丁
	resp, err := http.Get(url)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("补丁下载失败", err, url))
		return
	}
	defer resp.Body.Close()

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0x666)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("打开保存文件失败", err, filename))
		return
	}
	defer file.Close()

	if _, err = io.Copy(file, resp.Body); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("下载补丁文件失败", err, filename))
	}
}

//校验补丁
func (this *STPatchDown) CheckPatch(filename string) (string, bool) {
	//sha1
	idxL := strings.LastIndex(filename, "_")
	idxP := strings.LastIndex(filename, ".")
	if idxL == -1 || idxP == -1 {
		return "", false
	}
	shaVal := filename[idxL+1 : idxP]

	hfile, err := os.OpenFile(filename, os.O_RDONLY, 0x666)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("打开计算文件CRC失败", err, filename))
		return "", false
	}
	defer hfile.Close()

	if idxP-idxL == 32 {
		crc32new := md5.New()
		if _, err = io.Copy(crc32new, hfile); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("计算文件CRC失败", err, filename))
		}
		realSha := fmt.Sprintf(`%x`, crc32new.Sum(nil))
		return realSha, (realSha == shaVal)
	} else {
		sha1new := sha1.New()
		if _, err = io.Copy(sha1new, hfile); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("计算文件CRC失败", err, filename))
		}
		realSha := fmt.Sprintf(`%x`, sha1new.Sum(nil))
		return realSha, (realSha == shaVal)
	}
}

//校验补丁（改成CRC,并不校验原哈希值）
func (this *STPatchDown) CheckPatchEx(filename string) (string, bool) {
	hfile, err := os.OpenFile(filename, os.O_RDONLY, 0x666)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("打开计算补丁CRC失败", err, filename))
		return "", false
	}
	defer hfile.Close()

	crc32new := crc32.NewIEEE()
	if _, err = io.Copy(crc32new, hfile); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("计算文件CRC失败", err, filename))
		return "", true
	}
	realSha := fmt.Sprintf(`%v`, crc32new.Sum32())
	return realSha, true
}
