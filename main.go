package main

/*
文件结构

DIR-patchs
	-filter.txt
	-补丁.exe
	-补丁.msu
	-......
-main.exe
*/

import (
	"./patchdown"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
)

var StartType = flag.String("type", "", "")

func GetInfo() {
	//信息获取器
	log.Println("启动补丁信息获取...")
	patchdown.Main_getinfo()
}

func DownLoad() {
	//启动下载
	patchdown.GLogCollect.ToRunLog(fmt.Sprintln("启动补丁下载..."))
	patchdown.GSTPatchDown.Start()
}

func CopyFile() {
	log.Println("启动补丁拷贝...")
	if err := patchdown.StartWorker("2368"); err != nil {
		patchdown.GLogCollect.ToRunLog(fmt.Sprintln("启动补丁拷贝失败", err))
		os.Exit(0)
	}
}

func main() {
	flag.Parse()

	if err := patchdown.GLogCollect.Start(); err != nil {
		log.Println("初始化日志失败", err)
		return
	}
	if err := patchdown.GSqlOpt.Start(); err != nil {
		patchdown.GLogCollect.ToRunLog(fmt.Sprintln("链接数据库失败", err))
		return
	}

	cmdinfo := exec.Command("cmd.exe", "/C", "start", "net", "start", "ywPatchDownCenterSvr")
	if err := cmdinfo.Run(); err != nil {
		patchdown.GLogCollect.ToRunLog(fmt.Sprintln("启动服务失败"))
	}

	//一共两种模式：内网模式  外网模式
	patchdown.GLogCollect.ToRunLog(fmt.Sprintln("启动模式：", *StartType))
	if *StartType == "out" {
		//外网模式
		patchdown.GLogCollect.ToRunLog(fmt.Sprintln("外网模式：HTTP服务启动..."))
		go DownLoad() //下载补丁
		go CopyFile() //获取补丁信息
		GetInfo()     //导出补丁文件
	} else {
		//默认内网模式
		patchdown.GLogCollect.ToRunLog(fmt.Sprintln("内网模式：HTTP服务启动..."))
		patchdown.GSTStatMgr.Init()
		if err := patchdown.StartHttpServer("2368"); err != nil {
			patchdown.GLogCollect.ToRunLog(fmt.Sprintln("在2368端口启动侦听服务失败", err))
		}
	}
}
