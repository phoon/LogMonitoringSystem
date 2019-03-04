package main

import (
	"fmt"
	"strings"
	"sync"
)

//使用接口机制，增强程序可扩展性
type Reader interface {
	Read(rc chan string)
}

type Writer interface {
	Write(wc chan string)
}

type LogProcess struct {
	rc    chan string // 读取通道
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
func (r *ReadFromFile) Read(rc chan string) {
	line := "message"
	rc <- line
	wg.Done()
}

//写入模块
func (w *WriteToInfluxDB) Write(wc chan string) {
	fmt.Println(<-wc)
	wg.Done()
}

// 解析模块
func (l *LogProcess) Process() {
	data := <-l.rc
	l.wc <- strings.ToUpper(data)
	wg.Done()
}

var wg sync.WaitGroup

func main() {
	r := &ReadFromFile{
		path: "/tmp/access.log",
	}

	w := &WriteToInfluxDB{
		influxDBDsn: "username:password...",
	}

	lp := &LogProcess{
		rc:    make(chan string),
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
