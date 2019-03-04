package main

import (
	"fmt"
	"strings"
	"sync"
)

type LogProcess struct {
	rc          chan string // 读取通道
	wc          chan string // 写入通道
	path        string      // 读取文件的路径
	influxDBDsn string      // influx data source
}

// 读取模块
func (l *LogProcess) ReadFromFile() {
	line := "message"
	l.rc <- line
	wg.Done()
}

// 解析模块
func (l *LogProcess) Process() {
	data := <-l.rc
	l.wc <- strings.ToUpper(data)
	wg.Done()
}

// 写入模块
func (l *LogProcess) WriteToInfluxDB() {
	fmt.Println(<-l.wc)
	wg.Done()
}

var wg sync.WaitGroup

func main() {
	lp := &LogProcess{
		rc:          make(chan string),
		wc:          make(chan string),
		path:        "/tmp/access.log",
		influxDBDsn: "username:password...",
	}

	wg.Add(3)
	go lp.ReadFromFile()
	go lp.Process()
	go lp.WriteToInfluxDB()
	wg.Wait()
}
