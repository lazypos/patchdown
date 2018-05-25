package patchdown

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	SQL_LOAD_LASTOUTPORTTIME = `Call EditExportTime('%v')`
)

//获取上一次的补丁导出时间
func GetLastPatchOutportTime() string {
	sql := fmt.Sprintf(SQL_LOAD_LASTOUTPORTTIME, "")
	s, err := GSqlOpt.QueryVal(sql)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("获取上一次导出补丁时间失败.", err, sql))
		return fmt.Sprintf(`获取失败,%v`, err)
	}
	if len(s) == 0 {
		return "从未导出"
	}
	return s
}

func HandleCopy(c *gin.Context) {
	GLogCollect.ToRunLog(fmt.Sprintln("系统被访问...", c.ClientIP()))

	if !strings.Contains(c.ClientIP(), "127.0.0.1") && !strings.Contains(c.ClientIP(), "::") {
		GLogCollect.ToRunLog(fmt.Sprintln("非本地访问", c.ClientIP()))
		fmt.Fprintf(c.Writer, "请在本机使用127.0.0.1访问")
		return
	}

	st := &struct {
		Version  string
		LastTime string
	}{LastTime: GetLastPatchOutportTime(), Version: GVersion}
	c.HTML(http.StatusOK, "copyfile.tpl", st)
}

var GMuxCopy sync.Mutex
var GCopyProcess int = 0
var GError error = nil
var GTotal int = 0

//处理进度查询
func HandleProcess(c *gin.Context) {
	if GError != nil {
		c.JSON(http.StatusOK, gin.H{
			"error":   GError.Error(),
			"message": "",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"error":   "",
		"message": fmt.Sprintf(`%v%%`, GCopyProcess%101),
		"info":    fmt.Sprintf(`补丁导出完成，共导出%v个补丁`, GTotal),
	})
}

//获取需要导出的补丁列表
func GetPatchList(bgdate, eddate string) ([]string, error) {
	arrRst := []string{}

	patchDir := GetPatchStoreDir()
	if len(patchDir) == 0 {
		GLogCollect.ToRunLog(fmt.Sprintln("补丁下载目录错误", patchDir))
		return arrRst, fmt.Errorf(`补丁下载目录错误 %v`, patchDir)
	}

	err := filepath.Walk(patchDir, func(fpath string, finfo os.FileInfo, err error) error {
		if finfo.IsDir() || !CheckIsPatchFile(finfo.Name()) {
			return nil
		}
		//比较文件时间
		fileTime := finfo.ModTime().String()[:19]
		if len(bgdate) > 0 && fileTime < bgdate {
			return nil
		}
		if len(eddate) > 0 && fileTime > eddate {
			return nil
		}

		arrRst = append(arrRst, fpath)
		return nil
	})
	return arrRst, err
}

//补丁导出操作
func CopyPatchs(arrPatchs []string, dst, stime string) error {
	total := len(arrPatchs)
	GTotal = total
	for i, patch := range arrPatchs {

		//拷贝文件
		df, err := os.Create(filepath.Join(dst, filepath.Base(patch)))
		if err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("拷贝文件出错 创建目的文件失败", err, dst))
			GError = fmt.Errorf(`拷贝补丁失败,创建目的文件失败,%v,%v`, err, dst)
			return err
		}
		defer df.Close()

		sf, err := os.Open(patch)
		if err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("拷贝文件出错 打开源文件失败", err))
			GError = fmt.Errorf(`拷贝补丁失败,打开源文件失败,%v,%v`, err, patch)
			return err
		}
		defer sf.Close()

		if _, err = io.Copy(df, sf); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("拷贝文件出错", err))
			GError = fmt.Errorf(`拷贝文件出错,%v,%v`, err, patch)
			return err
		}

		GCopyProcess = int(i * 100 / total)
	}
	GLogCollect.ToRunLog(fmt.Sprintln("导出补丁文件结束..."))
	GCopyProcess = 100
	//增量导出则记录当前时间
	if len(stime) > 0 {
		if err := GSqlOpt.Execute(fmt.Sprintf(SQL_LOAD_LASTOUTPORTTIME, time.Now().String()[:19])); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("记录上一次导出时间出错", err))
			return err
		}
	}
	return nil
}

//补丁导出函数
func HandleCopyFile(c *gin.Context) {
	GLogCollect.ToRunLog(fmt.Sprintln("收到导出补丁请求", c.Request.RequestURI))
	GMuxCopy.Lock()
	defer GMuxCopy.Unlock()

	cDest := c.Request.FormValue("dest") //导出位置
	fmt.Println(cDest)
	if len(cDest) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "导出位置不能为空.",
		})
		return
	}
	//导出类型（1：全 2：增 3：自定义）
	cType := c.Request.FormValue("type")
	bgtime := ""
	edtime := ""
	stime := ""
	if cType == "1" {
		GLogCollect.ToRunLog(fmt.Sprintln("全量导出补丁。"))
	} else if cType == "2" {
		stime = GetLastPatchOutportTime()
		bgtime = stime
		GLogCollect.ToRunLog(fmt.Sprintln("将从", stime, "开始导出补丁。"))
	} else {
		cTime := c.Request.FormValue("time")
		if len(cTime) == 0 || !CheckIsTimeRange(cTime) {
			c.JSON(http.StatusOK, gin.H{
				"message": "请选择正确的导出时间范围.",
			})
			return
		}
		bgtime = cTime[:19]
		edtime = cTime[22:]
	}
	//获取导出补丁
	arrPatchs, err := GetPatchList(bgtime, edtime)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("获取导出补丁个数出错", err))
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("获取导出补丁个数出错. %v", err),
		})
		return
	}
	GLogCollect.ToRunLog(fmt.Sprintln("需要导出补丁个数：", len(arrPatchs)))

	//开始导出补丁
	GCopyProcess = 0
	GError = nil
	GTotal = 0

	go CopyPatchs(arrPatchs, cDest, stime)
	c.JSON(http.StatusOK, gin.H{
		"message": "",
	})
}

func StartWorker(port string) error {
	GLogCollect.ToRunLog(fmt.Sprintln("启动补丁拷贝服务...", GVersion))

	r := gin.Default()
	r.LoadHTMLFiles(filepath.Join(GExePath, "/template/copyfile.tpl"))
	r.Static("/layui", filepath.Join(GExePath, "/layui"))
	r.GET("/process", HandleProcess)
	r.GET("/copy", HandleCopyFile)
	r.GET("/", HandleCopy)
	return r.Run(fmt.Sprintf(`:%v`, port))
}
