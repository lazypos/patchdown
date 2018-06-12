package patchdown

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type STSQLOperation struct {
	Conn *sql.DB
}

var GSqlOpt = &STSQLOperation{}

func (this *STSQLOperation) LoadDBConfig() (ip, dbname, user, pass string) {
	text, err := ioutil.ReadFile(filepath.Join(GExePath, "db.cfg"))
	if err == nil {
		arr := strings.Split(string(text), ";")
		if len(arr) == 4 {
			return arr[0], arr[1], arr[2], arr[3]
		}
	}
	GLogCollect.ToRunLog(fmt.Sprintln("读取数据库配置错误", err))
	return "", "", "", ""
}

func (this *STSQLOperation) Start() error {
	GLogCollect.ToRunLog(fmt.Sprintln(`初始化数据库连接...`))
	defer GLogCollect.ToRunLog(fmt.Sprintln(`初始化数据库连接完毕...`))

	this.Close()
	var err error
	//connStr := "server=172.16.3.80;user id=sa;password=123;database=ywtest;encrypt=disable"
	//connStr := "root:123456@tcp(172.16.5.77:3306)/test"
	ip, dbname, user, pass := this.LoadDBConfig()
	connStr := fmt.Sprintf(`%s:%s@tcp(%s)/%s`, user, pass, ip, dbname)
	this.Conn, err = sql.Open("mysql", connStr)
	if err != nil {
		GLogCollect.ToRunLog(fmt.Sprintln("连接数据库失败", connStr, err))
		return err
	}
	return nil
}

func (this *STSQLOperation) Execute(sql string) error {
	_, err := this.Conn.Exec(sql)
	return err
}

func (this *STSQLOperation) Query(sql string) (*sql.Rows, error) {
	rows, err := this.Conn.Query(sql)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (this *STSQLOperation) QueryVal(sql string) (string, error) {
	rows, err := this.Conn.Query(sql)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var rst string
	for rows.Next() {
		err = rows.Scan(&rst)
		return rst, nil
	}
	return rst, nil
}

func (this *STSQLOperation) Close() {
	if this.Conn != nil {
		this.Conn.Close()
		this.Conn = nil
	}
}

func (this *STSQLOperation) Begin() (*sql.Tx, error) {
	return this.Conn.Begin()
}
