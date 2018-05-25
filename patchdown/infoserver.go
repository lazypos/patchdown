//对外查询接口
package patchdown

import (
	"bytes"
	"fmt"
)

type STQueryInfo struct {
}

//查询补丁服务器所有的补丁文件和其一些信息
func (this *STQueryInfo) QueryInfo() string {
	m := GPatchFileMgr.GetPatchMap()
	buf := bytes.NewBufferString("")
	for _, st := range m {
		buf.WriteString(fmt.Sprintf("%s_%d\r\n", st.PatchName, st.Sha256OK))
	}
	return buf.String()
}

//其他查询接口
func (this *STQueryInfo) Query() string {
	return ""
}
