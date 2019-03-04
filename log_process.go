package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

//使用接口机制，增强程序可扩展性
type Reader interface {
	Read(rc chan []byte)
}

type Writer interface {
	Write(wc chan string)
}

type LogProcess struct {
	rc    chan []byte // 读取通道
	wc    chan string // 写入通道
	read  Reader      //读取器
	write Writer      //写入器
}

type ReadFromFile struct {
	path string // 读取文件的路径
}

type WriteToInfluxDB struct {
	influxDBDsn string // influx data source
}

//读取模块
func (r *ReadFromFile) Read(rc chan []byte) {
	//打开文件
	f, err := os.Open(r.path)
	if err != nil {
		panic(fmt.Sprintf("open file error: %s", err.Error()))
	}

	//从文件末尾开始逐行读取文件內容
	//将文件指针移到末尾
	f.Seek(0, 2)

	rd := bufio.NewReader(f)

	for {
		line, err := rd.ReadBytes('\n')
		//文件末尾，等待后重新循环
		if err == io.EOF {
			//暂停500ms,以免长时间的CPU占用
			time.Sleep(500 * time.Microsecond)
			continue
		} else if err != nil {
			panic(fmt.Sprintf("ReadBytes error: %s", err.Error()))
		}
		//去除行尾换行符
		rc <- line[:len(line)-1]
	}

	wg.Done()
}

//写入模块
func (w *WriteToInfluxDB) Write(wc chan string) {
	for v := range wc {
		fmt.Println(v)
	}

	wg.Done()
}

// 解析模块
func (l *LogProcess) Process() {
	for v := range l.rc {
		l.wc <- strings.ToUpper(string(v))
	}

	wg.Done()
}

var wg sync.WaitGroup

func main() {
	r := &ReadFromFile{
		path: "./access.log",
	}

	w := &WriteToInfluxDB{
		influxDBDsn: "username:password...",
	}

	lp := &LogProcess{
		rc:    make(chan []byte),
		wc:    make(chan string),
		read:  r,
		write: w,
	}

	wg.Add(3)
	go lp.read.Read(lp.rc)
	go lp.Process()
	go lp.write.Write(lp.wc)
	wg.Wait()
}
