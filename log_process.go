package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/influxdb1-client/v2"
)

//使用接口机制，增强程序可扩展性
type Reader interface {
	Read(rc chan []byte)
}

type Writer interface {
	Write(wc chan *Message)
}

type LogProcess struct {
	rc    chan []byte   // 读取通道
	wc    chan *Message // 写入通道
	read  Reader        //读取器
	write Writer        //写入器
}

type ReadFromFile struct {
	path string // 读取文件的路径
}

type WriteToInfluxDB struct {
	influxDBDsn string // influx data source
}

//Message结构体用于存储提取出来的日志数据
type Message struct {
	TimeLocal                    time.Time
	BytesSent                    int
	Path, Method, Scheme, Status string
	UpstreamTime, RequestTime    float64
}

//系统状态监控
type SystemInfo struct {
	HandleLine   int     `json:"handleLine"`   //总处理日志行数
	Tps          float64 `json:"tps"`          //系统吞吐量
	ReadChanLen  int     `json:"readChanLen"`  //read channel 长度
	WriteChanLen int     `json:"writeChanLen"` //write channel 长度
	RunTime      string  `json:"runTime"`      //运行总时间
	ErrNum       int     `json:"errNum"`       //错误数
}

const (
	TypeHandleLine = 0
	TypeErrNum     = 1
)

var TypeMonitorChan = make(chan int, 200)

//监控模块封装
type Monitor struct {
	startTime time.Time
	data      SystemInfo
	tpsSli    []int //存储获取的数据
}

func (m *Monitor) start(lp *LogProcess) {

	go func() {
		for n := range TypeMonitorChan {
			switch n {
			case TypeErrNum:
				m.data.ErrNum += 1
			case TypeHandleLine:
				m.data.HandleLine += 1
			}
		}
	}()

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			<-ticker.C
			m.tpsSli = append(m.tpsSli, m.data.HandleLine)
			if len(m.tpsSli) > 2 {
				m.tpsSli = m.tpsSli[1:]
			}
		}
	}()

	http.HandleFunc("/monitor", func(w http.ResponseWriter, r *http.Request) {
		m.data.RunTime = time.Now().Sub(m.startTime).String()
		m.data.ReadChanLen = len(lp.rc)
		m.data.WriteChanLen = len(lp.wc)

		if len(m.tpsSli) >= 2 {
			m.data.Tps = float64(m.tpsSli[1]-m.tpsSli[0]) / 5
		}

		ret, _ := json.MarshalIndent(m.data, "", "\t")

		io.WriteString(w, string(ret))
	})

	http.ListenAndServe(":9193", nil)
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
		//每读到一行数据，就添加一个标记
		TypeMonitorChan <- TypeHandleLine
		//去除行尾换行符
		rc <- line[:len(line)-1]
	}

	wg.Done()
}

//写入模块
func (w *WriteToInfluxDB) Write(wc chan *Message) {
	/*
		InfluxDB关键概念（与传统数据库对比）：
		database：数据库
		measurement：数据库中的表
		points：表里的一行数据（包含如下属性）：
				 ·-- tags：各种有索引的属性
				 |
		points --·-- fields：各种记录的值
				 |
				 ·-- time：数据记录的时间戳，也是自动生成的主索引
	*/

	//解析influxDBDsn
	infSli := strings.Split(w.influxDBDsn, "@")

	//创建一个新的 HTTPClient
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     infSli[0],
		Username: infSli[1],
		Password: infSli[2],
	})

	if err != nil {
		log.Fatal(err)
	}

	for v := range wc {
		//创建一个新的point batch
		bp, err := client.NewBatchPoints(client.BatchPointsConfig{
			Database:  infSli[3],
			Precision: infSli[4], //精度为秒
		})
		if err != nil {
			log.Fatal(err)
		}

		//创建一个新的point 并且添加至batch
		tags := map[string]string{"Path": v.Path, "Method": v.Method, "Scheme": v.Scheme, "Status": v.Status}
		fields := map[string]interface{}{
			"UpstreamTime": v.UpstreamTime,
			"RequestTime":  v.RequestTime,
			"BytesSent":    v.BytesSent,
		}

		//表名, tags, fields, time
		pt, err := client.NewPoint("nginx_log", tags, fields, v.TimeLocal)
		if err != nil {
			log.Fatal(err)
		}
		bp.AddPoint(pt)

		//写入batch
		if err := c.Write(bp); err != nil {
			log.Fatal(err)
		}

		log.Println("write success!")
	}

	wg.Done()
}

// 解析模块
func (l *LogProcess) Process() {
	/*
		172.0.0.2 - - [04/Mar/2018:13:49:52 +0000] http "GET /foo?query=t HTTP/1.0" 200 2133 "-" "KeepAliveClient" "-" 1.005 1.854
		对应正则表达式：
		([\d\,]+)\s+([^ \[]+)\s+([^ \[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s+([\d\.-]+)
	*/

	r := regexp.MustCompile(`([\d\,]+)\s+([^ \[]+)\s+([^ \[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s+([\d\.-]+)`)

	for v := range l.rc {
		ret := r.FindStringSubmatch(string(v))

		if len(ret) != 14 {
			//记录错误
			TypeMonitorChan <- TypeErrNum
			log.Println("FindStringSubmatch fail:", string(v))
			continue
		}

		message := &Message{}
		//解析message.TimeLocal
		loc, _ := time.LoadLocation("Asia/Shanghai")
		t, err := time.ParseInLocation("02/Jan/2006:15:04:05 +0000", ret[4], loc)
		if err != nil {
			//记录错误
			TypeMonitorChan <- TypeErrNum
			log.Println("ParseInLocation fail:", err.Error(), ret[4])
			continue
		}
		message.TimeLocal = t

		//解析message.BytesSent
		byteSent, _ := strconv.Atoi(ret[8])
		message.BytesSent = byteSent

		//解析message.Method
		// GET /foo?query=t HTTP/1.0
		reqSli := strings.Split(ret[6], " ")
		if len(reqSli) != 3 {
			//记录错误
			TypeMonitorChan <- TypeErrNum
			log.Println("strings.Split fail:", ret[6])
			continue
		}
		message.Method = reqSli[0]

		//解析message.Path
		u, err := url.Parse(reqSli[1])
		if err != nil {
			//记录错误
			TypeMonitorChan <- TypeErrNum
			log.Println("url parse fail:", err)
			continue
		}
		message.Path = u.Path

		//解析message.Schema
		message.Scheme = ret[5]

		//解析message.Status
		message.Status = ret[7]

		//解析message.UpstreamTime
		upstreamTime, _ := strconv.ParseFloat(ret[12], 64)
		message.UpstreamTime = upstreamTime

		//解析message.RequestTime
		requestTime, _ := strconv.ParseFloat(ret[13], 64)
		message.RequestTime = requestTime

		l.wc <- message
	}

	wg.Done()
}

var wg sync.WaitGroup

func main() {
	//从命令行获取要读取的文件以及连接influxDB数据库的凭证
	var path, influDsn string
	flag.StringVar(&path, "path", "./access.log", "read file path")
	flag.StringVar(&influDsn, "influDsn", "http://localhost:8086@imooc@imoocpass@imooc@s", "influx data source")
	flag.Parse()

	r := &ReadFromFile{
		path: path,
	}

	w := &WriteToInfluxDB{
		//地址：端口@用户名@密码@数据库@精度
		influxDBDsn: influDsn,
	}

	lp := &LogProcess{
		rc:    make(chan []byte),
		wc:    make(chan *Message),
		read:  r,
		write: w,
	}

	wg.Add(3)
	go lp.read.Read(lp.rc)
	go lp.Process()
	go lp.write.Write(lp.wc)

	//开始监控
	m := &Monitor{
		startTime: time.Now(),
		data:      SystemInfo{},
	}
	m.start(lp)

	wg.Wait()
}
