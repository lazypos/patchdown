//下载专用
package patchdown

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

//检查请求与文件
func CheckLeggal(ua, url, dir string) (bool, int64) {
	if !strings.Contains(ua, "yw") || strings.Contains(url, "..") {
		log.Println("非法请求.")
		return false, 0
	}

	if len(dir) > 0 {
		filepos := filepath.Join(GExePath, dir)
		stat, err := os.Stat(filepos)
		if err != nil || stat.IsDir() {
			log.Println("请求文件不存在.", err)
			return false, 0
		}
		return true, stat.Size()
	}
	return true, 0
}

//日志下载
func LogDown(c *gin.Context) {
	atomic.AddInt32(&GNowTotalQuery, 1)

	logFile := c.Request.FormValue("log")
	GLogCollect.ToRunLog(fmt.Sprintln("请求下载日志", c.ClientIP(), logFile))

	if strings.Contains(logFile, "..") {
		GLogCollect.ToRunLog(fmt.Sprintln("请求下载日志非法", c.ClientIP(), logFile))
		c.String(http.StatusNotFound, "请求下载日志非法")
		return
	}

	fPath, err := GLogCollect.ZipRunLog()
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("请求下载日志失败", c.ClientIP(), err))
		c.String(http.StatusNotFound, "请求下载日志失败")
		return
	}

	handle, err := os.Open(fPath)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("打开日志文件失败", err, c.ClientIP(), logFile))
		c.String(http.StatusNotFound, "打开日志文件失败 %v", err.Error())
		return
	}
	defer handle.Close()
	if _, err = io.Copy(c.Writer, handle); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("复制日志文件失败", err, c.ClientIP(), logFile))
		c.String(http.StatusNotFound, "复制日志文件失败 %v", err.Error())
		return
	}
}

var GNowDownloading int32 = 0
var GNowTotalDown int32 = 0
var GNowTotalQuery int32 = 0
var GPatchDown int32 = 0

//补丁下载
func PatchDown(c *gin.Context) {
	atomic.AddInt32(&GNowTotalQuery, 1)

	url := c.Param("name")

	atomic.AddInt32(&GNowDownloading, 1)
	defer atomic.AddInt32(&GNowDownloading, -1)

	if !CheckIsPatchFile(url) || !strings.Contains(c.Request.UserAgent(), "yw") || strings.Contains(url, "..") {
		GLogCollect.ToRunLog(fmt.Sprintln("客户端非法下载补丁", c.ClientIP(), url, c.Request.UserAgent()))
		c.String(http.StatusNotFound, "非法下载补丁")
		return
	}

	//补丁过滤
	if GPatchFileMgr.IsFilterPatch(url) {
		c.String(http.StatusNotFound, "过滤的补丁")
		return
	}

	//下载补丁文件
	patchFile := GExePath + "/patchs/" + url
	fs, err := os.Stat(patchFile)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("补丁不存在", c.ClientIP(), url, c.Request.UserAgent(), patchFile))
		c.String(http.StatusNotFound, "补丁不存在")
		return
	}

	//HTTP头
	c.Writer.Header().Set("Content-Length", fmt.Sprintf(`%v`, fs.Size()))
	c.Writer.WriteHeader(http.StatusOK)

	handle, err := os.OpenFile(patchFile, os.O_RDONLY, 0x666)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("打开补丁文件失败", patchFile, err))
		c.String(http.StatusNotFound, "打开补丁文件失败")
		return
	}
	defer handle.Close()
	if _, err = io.Copy(c.Writer, handle); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("发送补丁文件失败", patchFile, err))
		c.String(http.StatusNotFound, "发送补丁文件失败")
		return
	}
	atomic.AddInt32(&GNowTotalDown, 1)
}

type STIndex struct {
	Version        string //程序版本
	StartTime      string //启动时间
	TotalDown      string //总下载量
	TotalQuery     string //总请求
	Downing        string //正在现在数
	LocalCounts    string //本地补丁数
	PatchTime      string //补丁索引更新时间
	LastImportTime string //补丁最后导入时间
	PatchDowns     string //补丁下载次数
}

func ShowInfo(c *gin.Context) {
	atomic.AddInt32(&GNowTotalQuery, 1)

	st := &STIndex{Version: GVersion, StartTime: GStartTime}
	st.LocalCounts = fmt.Sprintf("%v", GPatchFileMgr.GetPatchLocalPatchCounts())
	st.TotalDown = fmt.Sprintf("%v", atomic.LoadInt32(&GNowTotalDown))
	st.Downing = fmt.Sprintf("%v", atomic.LoadInt32(&GNowDownloading))
	st.TotalQuery = fmt.Sprintf("%v", atomic.LoadInt32(&GNowTotalQuery))
	st.PatchTime = GSTStatMgr.GetXmlDate()
	st.PatchDowns = fmt.Sprintf("%v", atomic.LoadInt32(&GPatchDown))
	c.HTML(http.StatusOK, "index.tpl", st)
}

//查询各种信息
func QueryInfo(c *gin.Context) {
	atomic.AddInt32(&GNowTotalQuery, 1)
}

//查詢補丁文件的CRC值與下載補丁索引文件
func QueryPatchxml(c *gin.Context) {
	atomic.AddInt32(&GNowTotalQuery, 1)

	//查询CRC
	act := c.Param("action")
	if act == "/crc" {
		//查詢CRC
		c.String(http.StatusOK, "crc=%v", GSTStatMgr.GetXmlCrc())
		return
	}

	//下載補丁索引
	patchFile := filepath.Join(GExePath, "patchxml.zip")
	fs, err := os.Stat(patchFile)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("补丁索引不存在", c.ClientIP(), c.Request.UserAgent()))
		c.String(http.StatusNotFound, "补丁索引不存在")
		return
	}

	//HTTP头
	c.Writer.Header().Set("Content-Length", fmt.Sprintf(`%v`, fs.Size()))
	c.Writer.Header().Set("Content-Disposition", "attachment;filename=patchxml.zip")
	c.Writer.WriteHeader(http.StatusOK)

	handle, err := os.OpenFile(patchFile, os.O_RDONLY, 0x666)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("打开补丁索引文件失败", patchFile, err))
		c.String(http.StatusNotFound, "打开补丁索引文件失败")
		return
	}
	defer handle.Close()
	if _, err = io.Copy(c.Writer, handle); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("发送补丁索引文件失败", patchFile, err))
		c.String(http.StatusNotFound, "发送补丁索引文件失败")
		return
	}
	atomic.AddInt32(&GPatchDown, 1)
}

var (
	GVersion   = "0.18"
	GStartTime = ""
)

func HandleNoRouter(c *gin.Context) {
	GLogCollect.ToRunLog(fmt.Sprintln("非法访问", c.Request.RequestURI))
	c.String(http.StatusNotFound, "非法访问")
}

//开启下载服务
func StartHttpServer(port string) error {
	if len(port) == 0 {
		port = "2368"
	}
	GPatchFileMgr.Init(filepath.Join(GExePath, "patchs"))

	eg := gin.Default()
	eg.LoadHTMLFiles(filepath.Join(GExePath, "/template/index.tpl"))
	eg.NoRoute(HandleNoRouter)
	eg.StaticFile("/favicon.png", filepath.Join(GExePath, "/template/favicon.png"))
	eg.GET("/logdown", LogDown)
	eg.GET("/queryinfo", QueryInfo)
	eg.GET("/xml/*action", QueryPatchxml)
	eg.GET("/patch/:name", PatchDown)
	eg.GET("/", ShowInfo)

	GLogCollect.ToRunLog(fmt.Sprintln("内网补丁分发服务启动...", GVersion))
	GStartTime = time.Now().String()[:19]

	return eg.Run(fmt.Sprintf(":%v", port))
}
