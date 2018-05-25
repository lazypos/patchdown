//用于补丁文件管理

package patchdown

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type PatchInfo struct {
	PatchName string //补丁名
	Sha256OK  bool   //sha256的值是否正确
}

type PatchFileMgr struct {
	MapPatchs map[string]*PatchInfo
	MapFilter map[string]bool
	Abnormal  int
}

var GPatchFileMgr = &PatchFileMgr{}

//初始化
func (this *PatchFileMgr) Init(patchDir string) error {
	this.Abnormal = 0
	if err := this.FindPatchs(patchDir); err != nil {
		return err
	}
	filterFile := patchDir + "fliter.txt"
	if err := this.LoadFilterPatch(filterFile); err != nil {
		return err
	}
	return nil
}

func (this *PatchFileMgr) WalkFunction(path string, finfo os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if finfo.IsDir() {
		return nil
	}
	idxL := strings.LastIndex(path, "_")
	idxP := strings.LastIndex(path, ".")
	if idxL == -1 || idxP == -1 || idxP-idxL != 41 {
		return nil
	}

	shaVal := path[idxL+1 : idxP]
	text, err := ioutil.ReadFile(path)
	if err != nil {
		log.Println("读取补丁文件错误", err)
		return err
	}
	realSha := fmt.Sprintf(`%x`, sha1.Sum(text))
	st := &PatchInfo{}
	st.PatchName = finfo.Name()
	if shaVal == realSha {
		st.Sha256OK = true
	} else {
		st.Sha256OK = false
		this.Abnormal += 1
	}

	this.MapPatchs[finfo.Name()] = st
	return nil
}

//遍历所有本地补丁
func (this *PatchFileMgr) FindPatchs(patchDir string) error {
	this.MapPatchs = make(map[string]*PatchInfo)

	if err := filepath.Walk(patchDir, this.WalkFunction); err != nil {
		log.Println("遍历补丁文件夹错误", err)
		return err
	}
	return nil
}

func (this *PatchFileMgr) GetPatchMap() map[string]*PatchInfo {
	return this.MapPatchs
}

func (this *PatchFileMgr) GetPatchLocalPatchCounts() int {
	return len(this.MapPatchs)
}

//屏蔽的补丁列表
func (this *PatchFileMgr) LoadFilterPatch(filterFile string) error {
	this.MapFilter = make(map[string]bool)
	text, err := ioutil.ReadFile(filterFile)
	if err != nil {
		log.Println("加载过滤补丁文件失败.", err)
		return err
	}
	arrLines := strings.Split(string(text[:]), "\r\n")
	for _, l := range arrLines {
		this.MapFilter[l] = true
	}
	return nil
}

func (this *PatchFileMgr) IsFilterPatch(filepatch string) bool {
	_, ok := this.MapFilter[filepatch]
	return ok
}
