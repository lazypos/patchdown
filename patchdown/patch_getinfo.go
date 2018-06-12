package patchdown

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const (
	SQL_GETALLKID         = `select KBID from Dict_PatchInfo`
	SQL_INSERT_PATCHFILES = `insert into Dict_PatchFile (KBID,FileName,OS,Arch,Ver,Language,URL,Size,IsExist,Guid)
							  values('%v','%v','%v','%v','%v','%v','%v','%v','0','%v')`
	SQL_INSERT_PATCHINFO = `insert into Dict_PatchInfo (KBID, MsId,Title,Pubdate,Type,WarnLevel,Description,Webpage,CreateTime,Affects)
							 values('%v','%v','%v','%v','%v','%v','%v','%v','%v','%v')`
	SQL_TRUNCATE_REPALCEGUID = `truncate table Dict_PatchReplace`
	SQL_SELECT_REPALCEGUID   = `select Guid, ReplaceGuid from Dict_PatchReplace`
	SQL_UPDATE_REPALCEGUID   = `update Dict_PatchFile set Disabled=1 where Guid in (%v)`
	SQL_INSERT_REPALCEGUID   = `insert into Dict_PatchReplace (Guid,ReplaceGuid)values('%v','%v')`
)

var GMapFilter = make(map[string]string)

//从数据库获取表补丁号
func GetAllKidFromDB() (map[string]bool, error) {
	mKids := make(map[string]bool)
	rows, err := GSqlOpt.Query(SQL_GETALLKID)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("查询现有补丁号失败", err))
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var kid string
		if err = rows.Scan(&kid); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("SCAN现有补丁号失败", err))
			return nil, err
		}
		mKids[kid] = true
	}
	return mKids, nil
}

//每个补丁文件的结构体
type STInfo struct {
	OSS   string //操作系统
	VERS  string //内核版本
	ARCHS string //CPU架构
	LANGS string //语言类型
	SOFT  string //软件名
	NAME  string //文件名
	SIZE  string //文件大小
	URLS  string //文件加载路径
	GUID  string //guid
}

type STPatchInfo struct {
	XML360ID    int    `xml:"id,attr"`    //360的ID
	XML360Level int    `xml:"level,attr"` //360等级
	XML360Desc  string `xml:"desc,attr"`  //360描述
	XML360Date  string `xml:"date,attr"`  //360发布日期

	GUID                string              //GUID
	PathInfoPage        string              //补丁信息页面
	PatchName           string              //补丁名
	LastModified        string              //最后修改时间
	PatchSize           string              //补丁大小
	Description         string              //描述
	Architecture        []string            //适用的CPU体系
	Classification      string              //分类(Service Pack)
	Supportedproducts   []string            //支持的产品
	Supportedlanguages  []string            //支持的语言
	MSRCNumber          string              //微软安全更新发布号 MS_014
	MSRCSeverity        string              //重要程度
	KBID                string              //KB2979596
	MoreInformation     string              //更多信息
	SupportUrl          string              //支持路径
	MapBeRapalceupdates map[string]string   //被其他补丁替换 [guid]title
	MapRapalceupdates   []string            //替换其他补丁 [guid]title
	Restart             string              //安装后是否需要重启
	Requestuserinput    string              //是否需要用户输入
	Exclusively         string              //是否专门安装
	Network             string              //需要连接网络
	UninstallNotes      string              //卸载描述
	UninstallSteps      string              //卸载步骤
	MapDownLoad         map[string][]string //下载路径 [语言][]URL
	IsMore              string              //超过一页的(page 1 of 1)

	FilePatchs  []*STInfo //补丁文件信息列表
	PatchType   int       //补丁类型 0-系统 1-office 2-soft
	RepalceGuid string    //补丁的替换关系字符串
}

const (
	fmt_query_URL    = `http://www.catalog.update.microsoft.com/Search.aspx?q=KB%v`                //查询接口
	fmt_patch_URL    = `http://www.catalog.update.microsoft.com/ScopedViewInline.aspx?updateid=%s` //补丁信息接口
	fmt_download_URL = `http://www.catalog.update.microsoft.com/DownloadDialog.aspx`               //获取补丁下载路径接口
	//提交获取补丁下载路径信息
	fmt_postform_STRING = `updateIDs=%%5B%%7B%%22size%%22%%3A0%%2C%%22languages%%22%%3A%%22%%22%%2C%%22uidInfo%%22%%3A%%22%s%%22%%2C%%22updateID%%22%%3A%%22%s%%22%%7D%%5D&updateIDsBlockedForImport=&wsusApiPresent=&contentImport=&sku=&serverName=&ssl=&portNumber=&version=`
)

//从360获取KID信息
func Get360KidFromXML(xmlPath string) ([]*STPatchInfo, error) {
	type UpdateXML struct {
		Update []*STPatchInfo `xml:"Updates>Update"`
	}

	content, err := ioutil.ReadFile(xmlPath)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("读取原始xml失败", err, xmlPath))
		return []*STPatchInfo{}, err
	}

	Upxml := &UpdateXML{}
	if err = xml.Unmarshal(content, Upxml); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("解析原始xml失败", err))
		return []*STPatchInfo{}, err
	}
	return Upxml.Update, nil
}

//搜索子串
func FindSubString(src, beg, end string, isSub bool) string {
	posbg := strings.Index(src, beg)
	if posbg == -1 {
		return ""
	}
	posed := strings.Index(src[posbg:], end)
	if posed == -1 {
		return ""
	}
	if isSub {
		return src[posbg+len(beg) : posbg+posed]
	}
	return src[posbg : posbg+posed+len(end)]
}

func QueryKBGuid(st *STPatchInfo) ([]*STPatchInfo, error) {
	resp, err := http.Get(fmt.Sprintf(fmt_query_URL, st.XML360ID))
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("连接微软补丁服务器失败：", err, st.XML360ID))
		return []*STPatchInfo{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("获取补丁信息失败：", err, st.XML360ID))
		return []*STPatchInfo{}, err
	}

	//解析内容
	content := string(body)
	subString := FindSubString(content, `<table class="resultsBorder resultsBackGround"`, `</table>`, false)
	if len(subString) == 0 {
		return []*STPatchInfo{}, fmt.Errorf("无结果")
	}

	//有更多结果
	isMore := "1"
	if strings.Index(content, `page 1 of 1`) > 0 {
		isMore = "0"
	}
	//拿GUID
	arrSTs := []*STPatchInfo{}
	arrItems := strings.Split(subString, `</tr>`)
	for _, s := range arrItems {
		if strings.Index(s, `<tr id="`) == -1 {
			continue
		}
		//不包含KB
		if strings.Index(content, fmt.Sprintf(`无 KB%v`, st.XML360ID)) > 0 {
			continue
		}
		gid := FindSubString(s, `goToDetails("`, `");`, true)
		GLogCollect.ToRunLog(fmt.Sprintln("获取补丁GUID列表成功", st.XML360ID, gid))
		newST := &STPatchInfo{GUID: gid, IsMore: isMore, XML360ID: st.XML360ID,
			XML360Level: st.XML360Level, XML360Desc: st.XML360Desc, XML360Date: st.XML360Date}
		arrSTs = append(arrSTs, newST)
	}
	return arrSTs, nil
}

//对补丁信息页面进行解析
func ParsePatchInfo(patch *STPatchInfo, src *string) {
	//左半
	patch.PatchName = strings.Trim(FindSubString(*src, `<span id="ScopedViewHandler_titleText">`, `</span>`, true), "\r\n ")
	patch.LastModified = strings.Trim(FindSubString(*src, `<span id="ScopedViewHandler_date">`, `</span>`, true), "\r\n ")
	patch.PatchSize = strings.Trim(FindSubString(*src, `<span id="ScopedViewHandler_size">`, `</span>`, true), "\r\n ")
	patch.Description = strings.Trim(FindSubString(*src, `<span id="ScopedViewHandler_desc">`, `</span>`, true), "\r\n ")
	arch := strings.Trim(FindSubString(*src, `Architecture:</span>`, `</div>`, true), "\r\n ")
	arch = strings.Replace(arch, " ", "", -1)
	arch = strings.Replace(arch, "\r\n", "", -1)
	patch.Architecture = strings.Split(arch, `,`)
	patch.Classification = strings.Trim(FindSubString(*src, `Classification:</span>`, `</div>`, true), "\r\n ")
	produce := strings.Trim(FindSubString(*src, `Supported products:</span>`, `</div>`, true), "\r\n ")
	produce = strings.Replace(produce, " ", "", -1)
	produce = strings.Replace(produce, "\r\n", "", -1)
	patch.Supportedproducts = strings.Split(produce, `,`)
	languages := strings.Trim(FindSubString(*src, `Supported languages:</span>`, `</div>`, true), "\r\n ")
	languages = strings.Replace(languages, " ", "", -1)
	languages = strings.Replace(languages, "\r\n", "", -1)
	patch.Supportedlanguages = strings.Split(languages, `,`)
	//右半
	patch.MSRCNumber = strings.Trim(FindSubString(*src, `MSRC Number:</span>`, `</div>`, true), "\r\n ")
	patch.MSRCSeverity = strings.Trim(FindSubString(*src, `<span id="ScopedViewHandler_msrcSeverity">`, `</span>`, true), "\r\n ")
	patch.KBID = strings.Trim(FindSubString(*src, `KB article numbers:</span>`, `</div>`, true), "\r\n ")
	moreInfo := strings.Trim(FindSubString(*src, `More information:</span>`, `</div>`, true), "\r\n ")
	patch.MoreInformation = strings.Trim(FindSubString(moreInfo, `">`, `</a>`, true), "\r\n ")
	supportInfo := strings.Trim(FindSubString(*src, `Support Url:</span>`, `</div>`, true), "\r\n ")
	patch.SupportUrl = strings.Trim(FindSubString(supportInfo, `">`, `</a>`, true), "\r\n ")
	//替换规则
	beRepStr := strings.Trim(FindSubString(*src, `This update has been replaced by the following updates:</span>`, `<span`, true), "\r\n ")
	arrBeRep := strings.Split(beRepStr, `style="`)
	if len(arrBeRep) > 1 {
		m := make(map[string]string)
		for _, s := range arrBeRep {
			if !strings.Contains(s, `href=`) {
				continue
			}
			guid := FindSubString(s, `ScopedViewInline.aspx?updateid=`, `'>`, true)
			desc := strings.Trim(FindSubString(s, `'>`, `</a>`, true), "\r\n ")
			m[guid] = strings.Replace(desc, "\n", "", -1)
		}
		patch.MapBeRapalceupdates = m
	}
	repStr := strings.Trim(FindSubString(*src, `This update replaces the following updates:</span>`, `<div id="languageBox"`, true), "\r\n ")
	arrRep := strings.Split(repStr, `style="`)
	if len(arrBeRep) > 1 {
		l := []string{}
		for _, s := range arrRep {
			desc := strings.Trim(FindSubString(s, `padding-bottom: 0.3em;">`, `</div>`, true), "\r\n ")
			l = append(l, strings.Replace(desc, "\n", "", -1))
		}
		patch.MapRapalceupdates = l
	}
	//安装信息
	patch.Restart = strings.Trim(FindSubString(*src, `ScopedViewHandler_rebootBehavior">`, `</span>`, true), "\r\n ")
	patch.Requestuserinput = strings.Trim(FindSubString(*src, `ScopedViewHandler_userInput">`, `</span>`, true), "\r\n ")
	patch.Exclusively = strings.Trim(FindSubString(*src, `ScopedViewHandler_installationImpact">`, `</span>`, true), "\r\n ")
	patch.Network = strings.Trim(FindSubString(*src, `ScopedViewHandler_connectivity">`, `</span>`, true), "\r\n ")
	note := strings.Trim(FindSubString(*src, `Uninstall Notes:</span>`, `</div>`, true), "\r\n ")
	patch.UninstallNotes = strings.Trim(FindSubString(note+"</end>", `ScopedViewHandler_rebootBehavior">`, `</end>`, true), "\r\n ")
	patch.UninstallSteps = strings.Trim(FindSubString(*src, `Uninstall Steps:</span>`, `</div>`, true), "\r\n ")
}

//获取单个补丁的信息
func GetPatchInfo(patch *STPatchInfo) (error, *STPatchInfo) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", patch.PathInfoPage, nil)
	req.Header.Add("Accept-Language", "zh-CN,zh;q=0.9,zh-TW;q=0.8,en;q=0.7")
	resp, err := client.Do(req)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("读取HTTP内容失败", err))
		return err, nil
	}

	//解析内容
	content := string(body)
	ParsePatchInfo(patch, &content)
	return nil, patch
}

//解析页面，获取其中的下载信息（URL地址）
func ParseDownloadURL(patch *STPatchInfo, src *string) {
	i := 0
	m := make(map[string][]string)
	for {
		url := strings.Trim(FindSubString(*src, fmt.Sprintf(`downloadInformation[0].files[%d].url = '`, i), `';`, true), "\r\n ")
		lan := strings.Trim(FindSubString(*src, fmt.Sprintf(`downloadInformation[0].files[%d].longLanguages = '`, i), `';`, true), "\r\n ")
		if len(url) == 0 || len(lan) == 0 {
			break
		}
		_, ok := m[lan]
		if !ok {
			m[lan] = []string{url}
		} else {
			m[lan] = append(m[lan], url)
		}
		i++
	}
	patch.MapDownLoad = m
}

//获取所有下载路径
func GetDonwloadURL(patch *STPatchInfo) (error, *STPatchInfo) {

	client := &http.Client{}
	req, err := http.NewRequest("POST", fmt_download_URL, strings.NewReader(fmt.Sprintf(fmt_postform_STRING, patch.GUID, patch.GUID)))
	req.Header.Add("Referer", "http://www.catalog.update.microsoft.com/DownloadDialog.aspx")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Origin", "http://www.catalog.update.microsoft.com")
	req.Header.Add("Upgrade-Insecure-Requests", "1")
	resp, err := client.Do(req)
	if err != nil {
		return err, nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("读取文件失败", err))
		return err, nil
	}

	//解析内容
	content := string(body)
	ParseDownloadURL(patch, &content)
	return nil, patch
}

var GFilterKID = ""

func LoadNoUseKID() {
	text, _ := ioutil.ReadFile(GExePath + "/ok.txt")
	GFilterKID = string(text[:])
}
func SaveNoUseKID(kdis int) {
	GFilterKID = fmt.Sprintf(`%s,%d,`, GFilterKID, kdis)
	ioutil.WriteFile(GExePath+"/ok.txt", []byte(GFilterKID), 0x666)
}

func IsNoUseKID(id int) bool {
	return strings.Contains(GFilterKID, fmt.Sprintf(`,%d,`, id))
}

//合并
func JoinText(tx1, tx2 string) string {
	if len(tx1) == 0 && len(tx2) > 0 {
		return tx2
	}
	if len(tx1) > 0 && len(tx2) == 0 {
		return tx1
	}
	if len(tx1) > 0 && len(tx2) > 0 {
		return tx1 + "," + tx2
	}
	return ""
}

//将同一个文件的不同补丁合并
func JoinPatchFile(arrFiles []*STInfo) []*STInfo {
	// map[补丁文件名][补丁文件信息]
	mtmp := make(map[string]*STInfo)
	for _, stf := range arrFiles {
		tstf, ok := mtmp[stf.NAME]
		if ok {
			tstf.ARCHS = JoinText(tstf.ARCHS, stf.ARCHS)
			tstf.LANGS = JoinText(tstf.LANGS, stf.LANGS)
			tstf.VERS = JoinText(tstf.VERS, stf.VERS)
			tstf.OSS = JoinText(tstf.OSS, stf.OSS)
			tstf.SOFT = JoinText(tstf.SOFT, stf.SOFT)
		} else {
			mtmp[stf.NAME] = stf
		}
	}
	//把map组合起来
	r := []*STInfo{}
	for _, v := range mtmp {
		r = append(r, v)
	}
	return r
}

//补丁合并(把KID一样的补丁合并) 并且入库
func PatchJoinAndToDB(arrPatchs []*STPatchInfo) error {
	GLogCollect.ToRunLog(fmt.Sprintln("替换关系入库"))
	//先入库替换关系
	for _, st := range arrPatchs {
		st1 := FileterContentEX(st)
		if st1 == nil {
			GLogCollect.ToRunLog(fmt.Sprintln("过滤的补丁", st.XML360ID))
			continue
		}
		//处理替换字串
		for guid, _ := range st.MapBeRapalceupdates {
			st.RepalceGuid += guid + ","
		}
		if len(st.RepalceGuid) == 0 {
			continue
		}
		sql := fmt.Sprintf(SQL_INSERT_REPALCEGUID, st.GUID, st.RepalceGuid)
		if err := GSqlOpt.Execute(sql); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("替换关系入库失败", st.XML360ID, sql))
		}
	}

	//再合并补丁
	GLogCollect.ToRunLog(fmt.Sprintln("开始合并补丁..."))
	m := make(map[int]*STPatchInfo)
	bAllFilter := true
	lastKid := 0
	for _, st := range arrPatchs {
		if st.XML360ID != lastKid {
			lastKid = st.XML360ID
		}
		//过滤补丁
		st1 := FileterContentEX(st)
		if st1 == nil {
			GLogCollect.ToRunLog(fmt.Sprintln("过滤的补丁", st.XML360ID))
			continue
		}

		bAllFilter = false
		//信息补全
		st = InformationPatchEX(st1)

		nst, ok := m[st.XML360ID]
		if !ok {
			m[st.XML360ID] = st
			continue
		}

		//文件信息解析成结构体
		for lan, arrUrls := range st.MapDownLoad {
			for _, url := range arrUrls {
				fls := ParseUrl(st.Supportedproducts, st.Architecture, lan+"="+url, url, st.PatchSize, st.GUID)
				if fls != nil {
					st.FilePatchs = append(st.FilePatchs, fls)
				}
			}
		}
		nst.FilePatchs = append(nst.FilePatchs, st.FilePatchs...)
	}
	if bAllFilter {
		SaveNoUseKID(lastKid)
		GLogCollect.ToRunLog(fmt.Sprintln("完全过滤的补丁", lastKid))
	}

	//记录入库
	GLogCollect.ToRunLog(fmt.Sprintln("开始补丁入库..."))
	for _, st := range m {
		//将文件名一样的补丁合并掉
		st.FilePatchs = JoinPatchFile(st.FilePatchs)
		if len(st.FilePatchs) == 0 {
			SaveNoUseKID(st.XML360ID)
			GLogCollect.ToRunLog(fmt.Sprintln("沒有补丁文件的补丁", lastKid))
		}
		if err := ParseFilesItemsEX(st); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("补丁入库失败", err, st.XML360ID))
		}
	}
	return nil
}

//对差异补丁处理入库
func CompareNewKidToDB(arrPatchs []*STPatchInfo, mKid map[string]bool) ([]*STPatchInfo, error) {
	arrRST := []*STPatchInfo{}
	for _, info := range arrPatchs {
		if _, ok := mKid[fmt.Sprint(info.XML360ID)]; ok {
			//GLogCollect.ToRunLog(fmt.Sprintln("补丁已存在忽略补丁", info.XML360ID))
			continue
		}
		if IsNoUseKID(info.XML360ID) {
			//GLogCollect.ToRunLog(fmt.Sprintln("查不到信息的KID", info.XML360ID))
			continue
		}
		//把补丁的所有GUID拿到
		GLogCollect.ToRunLog(fmt.Sprintln("获取补丁GUID列表", info.XML360ID))
		arrOneSTPatches, err := QueryKBGuid(info)
		if err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("获取补丁GUID列表失败", err, info.XML360ID))
			SaveNoUseKID(info.XML360ID)
			continue
		}
		//获取补丁信息和下载地址
		for _, st := range arrOneSTPatches {
			GLogCollect.ToRunLog(fmt.Sprintln("获取补丁信息", info.XML360ID, st.GUID))
			st.PathInfoPage = fmt.Sprintf(fmt_patch_URL, st.GUID)
			if err, st = GetPatchInfo(st); err != nil {
				GLogCollect.ToRunLog(fmt.Sprintln("获取补丁信息失败", err, info.XML360ID))
				continue
			}
			GLogCollect.ToRunLog(fmt.Sprintln("获取补丁下载信息", info.XML360ID, st.GUID))
			if err, st = GetDonwloadURL(st); err != nil {
				GLogCollect.ToRunLog(fmt.Sprintln("获取补丁下载路径失败", err, info.XML360ID))
				continue
			}
		}
		if err = PatchJoinAndToDB(arrOneSTPatches); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("补丁解析入库失败.", err))
			continue
		}
		arrRST = append(arrRST, info)
	}
	return arrRST, nil
}

func FileterContentEX(st *STPatchInfo) *STPatchInfo {
	//去掉过滤的补丁号
	if st.XML360ID == 2919355 {
		return nil
	}
	//去掉arch不需要的
	arch := strings.Join(st.Architecture, ",")
	if strings.EqualFold(arch, "IA64") || strings.EqualFold(arch, "ARM64") ||
		strings.EqualFold(arch, "ARM") {
		return nil
	}

	//去掉sp包
	if strings.Contains(st.Classification, "Service Pack") {
		return nil
	}
	//去掉操作系统不符的
	os := strings.Join(st.Supportedproducts, ",")
	if strings.EqualFold(os, "WindowsEmbeddedStandard7") || strings.EqualFold(os, "MicrosoftWorks9") ||
		strings.EqualFold(os, "Windows8Embedded") || strings.EqualFold(os, "WindowsGDR-DynamicUpdate") ||
		strings.EqualFold(os, "Works6-9Converter") || strings.EqualFold(os, "OfficeCommunicationsServer2007,OfficeCommunicationsServer2007R2") ||
		strings.EqualFold(os, "WindowsTechnicalPreview") || strings.EqualFold(os, "Silverlight") ||
		strings.EqualFold(os, "OfficeXP") || strings.EqualFold(os, "VisualStudio2005") ||
		strings.EqualFold(os, "WindowsXP64位版本2003") || strings.EqualFold(os, "WindowsXPEmbedded") {
		return nil
	}
	//去掉语言不符合的
	lang := strings.Join(st.Supportedlanguages, ",")
	if !strings.EqualFold(lang, "all") && !strings.Contains(lang, "Chinese") &&
		!strings.Contains(lang, "English") {
		return nil
	}
	return st
}

//补丁替换关系处理
func FileterReplaceEX() error {
	//查询出所有的GUID和替换关系
	//计算替换关系，得到被替换的guid列表
	//设置guid状态为被替换
	rows, err := GSqlOpt.Query(SQL_SELECT_REPALCEGUID)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("获取补丁替换关系失败", err))
		return err
	}
	defer rows.Close()

	// guid ->  替换的GUID列表
	mGuid := make(map[string][]string)
	for rows.Next() {
		var guid string
		var rguid string
		if err = rows.Scan(&guid, &rguid); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("SCAN获取替换关系失败", err))
			continue
		}
		mGuid[guid] = strings.Split(rguid, ",")
	}

	//得到一个替换guid列表
	arrGuid := []string{}
	for gid, arrrptgid := range mGuid {
		for _, pg := range arrrptgid {
			if len(pg) == 0 {
				continue
			}
			if _, ok := mGuid[pg]; ok {
				//GLogCollect.ToRunLog(fmt.Sprintln("补丁", gid, "被", pg, "替换"))
				arrGuid = append(arrGuid, fmt.Sprintf(`'%v'`, gid))
				break
			}
		}
	}
	//将列表的数据更新入库
	GLogCollect.ToRunLog(fmt.Sprintln("开始更新替换关系到数据库"))
	sql := fmt.Sprintf(SQL_UPDATE_REPALCEGUID, strings.Join(arrGuid, ","))
	if err = GSqlOpt.Execute(sql); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("更新替换关系到数据库失败", err, sql))
		return err
	}
	return nil
}

func InformationPatchEX(st *STPatchInfo) *STPatchInfo {
	//处理补丁类型字段
	st.PatchType = 0
	if strings.Contains(strings.Join(st.Supportedproducts, ","), `Office`) {
		st.PatchType = 1
	}
	//补全架构x86
	arch := strings.Join(st.Architecture, ",")
	if (strings.Contains(st.PatchName, "32 位") || strings.Contains(st.PatchName, "x86")) && arch == "n/a" {
		for _, v := range st.MapDownLoad {
			url := strings.Join(v, ",")
			if strings.Contains(url, "x86") && (!strings.Contains(url, "x64") &&
				!strings.Contains(url, "amd64")) {
				st.Architecture = []string{"x86"}
			}
		}
	}
	//amd64
	if (strings.Contains(st.PatchName, "64 位") || strings.Contains(st.PatchName, "x64")) && arch == "n/a" {
		for _, v := range st.MapDownLoad {
			url := strings.Join(v, ",")
			if !strings.Contains(url, "x86") && (strings.Contains(url, "x64") ||
				strings.Contains(url, "amd64")) {
				st.Architecture = []string{"amd64"}
			}
		}
	}
	return st
}

const (
	//内核版本
	TYPE_VER_50  = "5.0"  //2000
	TYPE_VER_51  = "5.1"  //XP
	TYPE_VER_52  = "5.2"  //2003/2003R2
	TYPE_VER_60  = "6.0"  //VISTA/2008
	TYPE_VER_61  = "6.1"  //7/2008R2
	TYPE_VER_62  = "6.2"  //8/2012
	TYPE_VER_63  = "6.3"  //8.1/2012R2
	TYPE_VER_64  = "6.4"  //10预览
	TYPE_VER_100 = "10.0" //10/2016

	//操作系统类型
	TYPE_OS_WIN2k      = "Windows2000"                         //2000
	TYPE_OS_WINXP      = "WindowsXP"                           //xp 32
	TYPE_OS_WINXP64    = "WindowsXPx64Edition"                 //xp 64
	TYPE_OS_WIN2003    = "WindowsServer2003"                   //2003
	TYPE_OS_WIN2003DE  = "WindowsServer2003,DatacenterEdition" //2003数据中心
	TYPE_OS_WINVISTA   = "WindowsVista"                        //vista
	TYPE_OS_WIN7       = "Windows7"                            //7
	TYPE_OS_WIN2008    = "WindowsServer2008"                   //2008
	TYPE_OS_WIN2008R2  = "WindowsServer2008R2"                 //2008r2
	TYPE_OS_WIN8       = "Windows8"                            //8
	TYPE_OS_WIN81      = "Windows8.1"                          //8.1
	TYPE_OS_WIN2012    = "WindowsServer2012"                   //2012
	TYPE_OS_WIN2012R2  = "WindowsServer2012R2"                 //2012r2
	TYPE_OS_WIN10      = "Windows10"                           //10
	TYPE_OS_WIN10LTSB  = "Windows10LTSB"                       //10企业
	TYPE_OS_WIN2016    = "WindowsServer2016"                   //2016
	TYPE_OS_OFFICE2003 = "Office2003"
	TYPE_OS_OFFICE2007 = "Office2007"
	TYPE_OS_OFFICE2010 = "Office2010"
	TYPE_OS_OFFICE2013 = "Office2013"
	TYPE_OS_OFFICE2016 = "Office2016"

	TYPE_LANG_FULL_EN  = "english"
	TYPE_LANG_FULL_CHS = "chinese(simplified)"
	TYPE_LANG_FULL_CHT = "chinese(traditional)"

	TYPE_LANG_EN  = "en"  //英语
	TYPE_LANG_CHS = "chs" //简中
	TYPE_LANG_CHT = "cht" //繁中

	TYPE_ARCH_X86 = "x86"
	TYPE_ARCH_X64 = "x64"

	//间隔符号
	sep_fuhao = "\t"
)

//优化补丁描述
func RepalceDesc(desc string) string {
	desc = strings.Replace(desc, "32 位版本", "", -1)
	desc = strings.Replace(desc, "64 位版本", "", -1)
	desc = strings.Replace(desc, "Windows Server 2008", "Windows", -1)
	desc = strings.Replace(desc, "Windows XP Service Pack 2", "Windows", -1)
	desc = strings.Replace(desc, "Windows XP x64 Edition SP2", "Windows", -1)
	desc = strings.Replace(desc, "Windows Server 2003 SP1 或 SP2", "Windows", -1)
	desc = strings.Replace(desc, "Windows Server 2003 x64 Edition SP2", "Windows", -1)
	desc = strings.Replace(desc, "Windows Server 2008 R2 Beta", "Windows", -1)
	desc = strings.Replace(desc, "Windows 7 Beta", "Windows", -1)
	desc = strings.Replace(desc, "Windows 7", "Windows", -1)
	desc = strings.Replace(desc, "Windows Server 2008 R2", "Windows", -1)
	desc = strings.Replace(desc, "Windows 8.1", "Windows", -1)
	desc = strings.Replace(desc, "Windows Server 2012 R2", "Windows", -1)
	desc = strings.Replace(desc, "Windows 7 和 Server 2008 R2", "Windows", -1)
	desc = strings.Replace(desc, "Windows Vista", "Windows", -1)
	desc = strings.Replace(desc, "Windows Server 2008", "Windows", -1)
	return desc
}

//URL是否是所有语言
func IsAllLang(url string) bool {
	if strings.Contains(url, "zh_") || strings.Contains(url, "_it_") {
		return false
	}
	if strings.Contains(url, "_kk_") || strings.Contains(url, "_ja_") {
		return false
	}
	if strings.Contains(url, "_ar_") || strings.Contains(url, "_ko_") {
		return false
	}
	if strings.Contains(url, "_cs_") || strings.Contains(url, "_nb_") {
		return false
	}
	if strings.Contains(url, "_da_") || strings.Contains(url, "_nl_") {
		return false
	}
	if strings.Contains(url, "_de_") || strings.Contains(url, "_pl_") {
		return false
	}
	if strings.Contains(url, "_el_") || strings.Contains(url, "_pt_") {
		return false
	}
	if strings.Contains(url, "_en_") || strings.Contains(url, "_ro_") {
		return false
	}
	if strings.Contains(url, "_es_") || strings.Contains(url, "_ru_") {
		return false
	}
	if strings.Contains(url, "_fi_") || strings.Contains(url, "_sv_") {
		return false
	}
	if strings.Contains(url, "_fr_") || strings.Contains(url, "_th_") {
		return false
	}
	if strings.Contains(url, "_he_") || strings.Contains(url, "_tr_") {
		return false
	}
	if strings.Contains(url, "_hi_") || strings.Contains(url, "_uk_") {
		return false
	}
	if strings.Contains(url, "_km_") || strings.Contains(url, "_tn_") {
		return false
	}
	if strings.Contains(url, "_hu_") || strings.Contains(url, "_vi_") {
		return false
	}
	if strings.Contains(url, "_hr_") || strings.Contains(url, "_as_") {
		return false
	}
	if strings.Contains(url, "_nn_") || strings.Contains(url, "_cy_") {
		return false
	}
	if strings.Contains(url, "_id_") || strings.Contains(url, "_zu_") {
		return false
	}
	if strings.Contains(url, "_is_") || strings.Contains(url, "_sl_") {
		return false
	}
	if strings.Contains(url, "_ta_") || strings.Contains(url, "_ur_") {
		return false
	}
	if strings.Contains(url, "_sq_") || strings.Contains(url, "_fil_") {
		return false
	}
	if strings.Contains(url, "_az_") || strings.Contains(url, "_pa_") {
		return false
	}
	if strings.Contains(url, "_ms_") || strings.Contains(url, "_or_") {
		return false
	}
	if strings.Contains(url, "_lt_") || strings.Contains(url, "_ne_") {
		return false
	}
	if strings.Contains(url, "_lb_") || strings.Contains(url, "_fa_") {
		return false
	}
	if strings.Contains(url, "_kn_") || strings.Contains(url, "_te_") {
		return false
	}
	if strings.Contains(url, "_ga_") || strings.Contains(url, "_ml_") {
		return false
	}
	if strings.Contains(url, "_uz_") || strings.Contains(url, "_si_") {
		return false
	}
	if strings.Contains(url, "_lv_") || strings.Contains(url, "_sk_") {
		return false
	}
	if strings.Contains(url, "_quz_") || strings.Contains(url, "_af_") {
		return false
	}
	if strings.Contains(url, "_bn_") || strings.Contains(url, "_bg_") {
		return false
	}
	if strings.Contains(url, "_mk_") || strings.Contains(url, "_et_") {
		return false
	}
	if strings.Contains(url, "_gu_") || strings.Contains(url, "_mr_") {
		return false
	}
	if strings.Contains(url, "_bs_") {
		return false
	}

	if strings.Contains(url, "_ara_") || strings.Contains(url, "_chs_") {
		return false
	}
	if strings.Contains(url, "_cht_") || strings.Contains(url, "_csy_") {
		return false
	}
	if strings.Contains(url, "_dan_") || strings.Contains(url, "_deu_") {
		return false
	}
	if strings.Contains(url, "_ell_") || strings.Contains(url, "_enu_") {
		return false
	}
	if strings.Contains(url, "_esn_") || strings.Contains(url, "_fin_") {
		return false
	}
	if strings.Contains(url, "_fra_") || strings.Contains(url, "_heb_") {
		return false
	}
	if strings.Contains(url, "_hun_") || strings.Contains(url, "_ita_") {
		return false
	}
	if strings.Contains(url, "_jpn_") || strings.Contains(url, "_kor_") {
		return false
	}
	if strings.Contains(url, "_nld_") || strings.Contains(url, "_nor_") {
		return false
	}
	if strings.Contains(url, "_plk_") || strings.Contains(url, "_ptb_") {
		return false
	}
	if strings.Contains(url, "_ptg_") || strings.Contains(url, "_rus_") {
		return false
	}
	if strings.Contains(url, "_sve_") || strings.Contains(url, "_trk_") {
		return false
	}
	return true
}

//获取语言类型
func GetLangType(url string) string {
	if strings.Contains(url, "zh_cn") || strings.Contains(url, "_chs_") || strings.Contains(url, TYPE_LANG_FULL_CHS) {
		return TYPE_LANG_FULL_CHS
	}
	if strings.Contains(url, "zh_hk") || strings.Contains(url, "zh_tw") ||
		strings.Contains(url, "_cht_") || strings.Contains(url, TYPE_LANG_FULL_CHT) {
		return TYPE_LANG_FULL_CHT
	}
	if strings.Contains(url, "_en_") || strings.Contains(url, "_enu_") || strings.Contains(url, TYPE_LANG_FULL_EN) {
		return TYPE_LANG_FULL_EN
	}
	return ""
}

//是否全操作系统
func IsOSAll(url string) bool {
	url = strings.Replace(url, "windowsupdate", "", -1)
	url = strings.Replace(url, "windowsmedia", "", -1)
	if !strings.Contains(url, "windows") && !strings.Contains(url, "srv2k3") &&
		!strings.Contains(url, "_xp_") {
		return true
	}
	if strings.Contains(url, "windows_") && !strings.Contains(url, "windows_2003") &&
		!strings.Contains(url, "windows_2000") {
		return true
	}
	return false
}

//确定补丁的操作系统
func FindUrlOS(arros []string, url string) ([]string, []string) {
	if IsOSAll(url) {
		return arros, []string{}
	}
	//URL是否输出这个操作系统
	arrOS := []string{}
	for _, os := range arros {
		ok := true
		switch os {
		case TYPE_OS_WIN2k:
			if !strings.Contains(url, "windows2000") && !strings.Contains(url, "windows_2000") {
				ok = false
			}
		case TYPE_OS_WINXP:
			if !strings.Contains(url, "windowsxp") && !strings.Contains(url, "_xp_") {
				ok = false
			}
		case TYPE_OS_WINXP64:
			if !strings.Contains(url, "windowsxp") && !strings.Contains(url, "windowsserver2003") {
				ok = false
			}
		case TYPE_OS_WIN2003:
			if !strings.Contains(url, "windows2003") && !strings.Contains(url, "windowsserver2003") && !strings.Contains(url, "srv2k3") {
				ok = false
			}
		case TYPE_OS_WINVISTA:
			if !strings.Contains(url, "windows6.0") {
				ok = false
			}
		case TYPE_OS_WIN7:
			if !strings.Contains(url, "windows6.1") {
				ok = false
			}
		case TYPE_OS_WIN2008:
			if !strings.Contains(url, "windows6.0") {
				ok = false
			}
		case TYPE_OS_WIN2008R2:
			if !strings.Contains(url, "windows6.1") {
				ok = false
			}
		case TYPE_OS_WIN8:
			if !strings.Contains(url, "windows6.2") && !strings.Contains(url, "windows8_") {
				ok = false
			}
		case TYPE_OS_WIN81:
			if !strings.Contains(url, "windows8.1") {
				ok = false
			}
		case TYPE_OS_WIN2012:
			if !strings.Contains(url, "windows6.2") && !strings.Contains(url, "windows8_") {
				ok = false
			}
		case TYPE_OS_WIN2012R2:
			if !strings.Contains(url, "windows8.1") {
				ok = false
			}
		case TYPE_OS_WIN10:
			if !strings.Contains(url, "windows10.0") {
				ok = false
			}
		case TYPE_OS_WIN10LTSB:
			if !strings.Contains(url, "windows10.0") {
				ok = false
			}
		case TYPE_OS_WIN2016:
			if !strings.Contains(url, "windows10.0") {
				ok = false
			}
		}
		if ok {
			arrOS = append(arrOS, os)
		}
	}

	arrVER := []string{}
	ver := ""
	if strings.Contains(url, "6.0") {
		ver = "6.0"
	} else if strings.Contains(url, "6.1") {
		ver = "6.1"
	} else if strings.Contains(url, "6.2") {
		ver = "6.2"
	} else if strings.Contains(url, "6.3") {
		ver = "6.3"
	} else if strings.Contains(url, "6.4") {
		ver = "6.4"
	} else if strings.Contains(url, "10.0") {
		ver = "10.0"
	}

	if len(ver) > 0 {
		arrVER = append(arrVER, ver)
	}
	return arrOS, arrVER
}

//分析一个补丁，返回一个结构体
func ParseUrl(arros, arrarch []string, url, urlsrc string, psize, guid string) *STInfo {
	st := &STInfo{}
	st.URLS = urlsrc
	st.GUID = guid

	//先规整URL，过滤语言和CPU架构
	url = strings.Replace(url, "-", "_", -1)
	url = strings.Replace(url, " ", "", -1)
	url = strings.ToLower(url)
	if strings.Contains(url, "embedded") || strings.Contains(url, "_ia64_") {
		return nil
	}

	//找出其操作系统
	arrOS, arrVER := FindUrlOS(arros, url)
	st.OSS = strings.Join(arrOS, ",")
	st.VERS = strings.Join(arrVER, ",")
	st.SIZE = psize

	//找出CPU架构
	archs := strings.Join(arrarch, ",")
	if strings.Contains(url, "x86") {
		st.ARCHS = "x86"
	} else if strings.Contains(url, "x64") || strings.Contains(url, "amd64") {
		st.ARCHS = "amd64"
	} else {
		if strings.Contains(archs, "x86") && strings.Contains(archs, "amd64") {
			st.ARCHS = ""
		}
		if strings.Contains(archs, "n/a") {
			st.ARCHS = ""
		}
		if strings.Contains(archs, "x86") && !strings.Contains(archs, "amd64") {
			st.ARCHS = "x86"
		}
		if !strings.Contains(archs, "x86") && strings.Contains(archs, "amd64") {
			st.ARCHS = "amd64"
		}
	}

	//找出语言类型
	arrkv := strings.Split(url, "=")
	if !strings.Contains(arrkv[0], "all") && !strings.Contains(arrkv[0], "english") && !strings.Contains(arrkv[0], "chinese(") {
		return nil
	}
	kv1all := IsAllLang(arrkv[1])
	//URL是全语言的
	if kv1all {
		if strings.Contains(arrkv[0], "all") {
			st.LANGS = ""
		} else {
			if !strings.Contains(arrkv[0], "_") {
				if arrkv[0] == TYPE_LANG_FULL_EN {
					st.LANGS = TYPE_LANG_EN
				}
				if arrkv[0] == TYPE_LANG_FULL_CHS {
					st.LANGS = TYPE_LANG_CHS
				}
				if arrkv[0] == TYPE_LANG_FULL_CHT {
					st.LANGS = TYPE_LANG_CHT
				}
			}
			if strings.Contains(arrkv[0], "_") {
				arrLANG := []string{}
				if strings.Contains(arrkv[0], TYPE_LANG_FULL_EN) {
					arrLANG = append(arrLANG, TYPE_LANG_EN)
				}
				if strings.Contains(arrkv[0], TYPE_LANG_FULL_CHS) {
					arrLANG = append(arrLANG, TYPE_LANG_CHS)
				}
				if strings.Contains(arrkv[0], TYPE_LANG_FULL_CHT) {
					arrLANG = append(arrLANG, TYPE_LANG_CHT)
				}
				if len(arrLANG) == 3 {
					st.LANGS = ""
				} else {
					st.LANGS = strings.Join(arrLANG, ",")
				}
			}
		}
	} else { //非全语言
		l := GetLangType(arrkv[1])
		if len(l) == 0 {
			return nil
		}
		if strings.Contains(arrkv[0], "all") && !strings.Contains(arrkv[0], l) {
			return nil
		}
		if l == TYPE_LANG_FULL_EN {
			st.LANGS = TYPE_LANG_EN
		}
		if l == TYPE_LANG_FULL_CHS {
			st.LANGS = TYPE_LANG_CHS
		}
		if l == TYPE_LANG_FULL_CHT {
			st.LANGS = TYPE_LANG_CHT
		}
	}

	//文件名
	pos := strings.LastIndex(url, "/")
	if pos == -1 {
		GLogCollect.ToRunLog(fmt.Sprintln("查找文件名失败", url))
		return nil
	}
	st.NAME = url[pos+1:]

	//软件就找出软件名
	posbg := strings.Index(url, "windowsmedia")
	if posbg != -1 {
		posed := strings.Index(url[posbg:], "_")
		if posed != -1 {
			st.SOFT = url[posbg : posbg+posed]
		}
	}
	return st
}

//去重OS
func ChangeOS(oss string) string {
	if len(oss) == 0 {
		return ""
	}
	arr := strings.Split(oss, ",")
	m := make(map[string]int)
	for _, v := range arr {
		if len(v) == 0 || v == "DatacenterEdition" {
			continue
		}
		m[v] = 1
	}
	r := ""
	for k, _ := range m {
		r = r + k + ","
	}
	return r
}

func ParseFilesItemsEX(st *STPatchInfo) error {
	//入库，先如文件表再录KID表
	tx, err := GSqlOpt.Begin()
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("数据库启动事物失败.", err))
		return err
	}
	defer tx.Rollback()

	for _, fps := range st.FilePatchs {
		fps.OSS = ChangeOS(fps.OSS)
		sql := fmt.Sprintf(SQL_INSERT_PATCHFILES, st.XML360ID, fps.NAME, fps.OSS, fps.ARCHS, fps.VERS, fps.LANGS, fps.URLS, fps.SIZE, st.GUID)
		_, err = tx.Exec(sql)
		if err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("补丁文件信息入库失败.", err, sql))
			return err
		}
	}
	//再入库KID表
	sql := fmt.Sprintf(SQL_INSERT_PATCHINFO, st.XML360ID, st.MSRCNumber, st.XML360Desc, st.XML360Date, st.PatchType, st.XML360Level, RepalceDesc(st.Description), st.SupportUrl, time.Now().String()[:19], st.Classification)
	_, err = tx.Exec(sql)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("补丁基本信息入库失败.", err, sql))
		return err
	}
	if err = tx.Commit(); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("数据库提交事物失败.", err))
		return err
	}
	return nil
}

//做整个流程
func DoGetInfoWorking(xmlPath string) error {
	//先从数据库和xml文件中获取补丁列表并比较差异
	mPatchDB, err := GetAllKidFromDB()
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("数据库获取补丁列表失败.", err))
		return err
	}
	arrPatch360, err := Get360KidFromXML(xmlPath)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("原始索引获取补丁列表失败.", err))
		return err
	}
	_, err = CompareNewKidToDB(arrPatch360, mPatchDB)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("处理差异补丁失败.", err))
		return err
	}
	//对替换关系进行处理
	GLogCollect.ToRunLog(fmt.Sprintln("开始处理替换关系..."))
	if err = FileterReplaceEX(); err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("处理替换关系失败.", err))
		return err
	}
	return nil
}

func Get360XMLPath() string {
	SQL_SELECT_XMLPATH := `Call GetConfig('360_DIR')`
	r, err := GSqlOpt.QueryVal(SQL_SELECT_XMLPATH)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("查询xml所在位置失败.", err))
		return ""
	}
	return r
}

//主函数
func Main_getinfo() error {
	log.Println("Main_getinfo")
	//循环下载
	for {
		GLogCollect.ToRunLog(fmt.Sprintln("开启新一轮的信息获取."))

		LoadNoUseKID()
		xmlpath := Get360XMLPath()
		if len(xmlpath) == 0 {
			GLogCollect.ToRunLog(fmt.Sprintln("无法获取xml所在位置"))
			time.Sleep(time.Second * 60)
			continue
		}
		GLogCollect.ToRunLog(fmt.Sprintln("補丁xml位置:", xmlpath))

		if err := DoGetInfoWorking(filepath.Join(xmlpath, "leakrepair.dat")); err != nil {
			GLogCollect.ToRunLog(fmt.Sprintln("本轮获取补丁信息失败.", err))
		}
		GLogCollect.ToRunLog(fmt.Sprintln("完成一轮的信息获取."))
		time.Sleep(time.Second * 60)
	}
	return nil
}
