package patchdown

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

var GExePath = GetExePath()

func GetExePath() string {
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return dir
}

//压缩文件
func CompressFile2Zip(srcFile, dstFile string) error {
	handleZip, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer handleZip.Close()

	zw := zip.NewWriter(handleZip)
	defer zw.Close()

	handle, err := zw.Create(filepath.Base(srcFile))
	if err != nil {
		return err
	}
	//将源文件写入zip文件
	handleSrc, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer handleSrc.Close()
	_, err = io.Copy(handle, handleSrc)
	if err != nil {
		return err
	}
	return zw.Flush()
}

//解压文件
func UncompressZip(srcFile, dstDir string) error {
	zr, err := zip.OpenReader(srcFile)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, vfile := range zr.File {
		if vfile.FileInfo().IsDir() {
			err := os.MkdirAll(dstDir+vfile.Name, 0644)
			if err != nil {
				return err
			}
			continue
		}
		//文件解压
		srcFile, err := vfile.Open()
		if err != nil {
			return err
		}
		defer srcFile.Close()

		newFile, err := os.Create(dstDir + vfile.Name)
		if err != nil {
			return err
		}
		defer newFile.Close()

		if _, err = io.Copy(newFile, srcFile); err != nil {
			return err
		}
	}
	return nil
}

//解壓文件到字符串
func UncompressZipToString(srcFile string) (string, error) {
	zr, err := zip.OpenReader(srcFile)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	//文件解压
	sFile, err := zr.File[0].Open()
	if err != nil {
		return "", err
	}
	defer sFile.Close()

	buf := bytes.NewBuffer(nil)
	if _, err = io.Copy(buf, sFile); err != nil {
		return "", err
	}
	return buf.String(), nil
}

//遞歸壓縮
func CompressRecu(srcFile, deepDir string, zw *zip.Writer) error {
	fInfo, err := os.Stat(srcFile)
	if err != nil {
		return err
	}

	if fInfo.IsDir() {
		newDir := filepath.Join(deepDir, fInfo.Name())
		arrFiles, err := ioutil.ReadDir(srcFile)
		if err != nil {
			return err
		}

		for _, vfile := range arrFiles {
			fullPath := filepath.Join(srcFile, vfile.Name())
			err = CompressRecu(fullPath, newDir, zw)
			if err != nil {
				return err
			}
		}
		return nil
	}

	//压缩文件
	handle, err := zw.Create(filepath.Join(deepDir, filepath.Base(srcFile)))
	if err != nil {
		return err
	}

	handleSrc, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer handleSrc.Close()
	_, err = io.Copy(handle, handleSrc)
	if err != nil {
		return err
	}

	return nil
}

//文件夹压缩
func CompressDir2Zip(srcDir, dstFile string) error {
	handleZip, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer handleZip.Close()

	zw := zip.NewWriter(handleZip)
	defer zw.Close()

	return CompressRecu(srcDir, "", zw)
}

//检查是否是补丁名的格式
func CheckIsPatchFile(filename string) bool {
	ok, err := regexp.MatchString("(.*?)_[0-9a-f]{15,}\\.[a-z]{3}", filename)
	log.Println(err, ok, filename)

	return ok
}

//是否符合时间格式
func CheckIsTimeRange(timerange string) bool {
	ok, _ := regexp.MatchString("[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2} - [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}", timerange)
	//log.Println(err, ok)
	return ok
}

//获取补丁下载目录
func GetPatchStoreDir() string {
	SQL_SELECT_DIR := `Call GetConfig('PATCH_DIR')`
	r, err := GSqlOpt.QueryVal(SQL_SELECT_DIR)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("查询补丁下载所在位置失败.", err, SQL_SELECT_DIR))
		return ""
	}
	return r
}
