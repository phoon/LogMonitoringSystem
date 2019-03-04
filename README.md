# 日志监控系统

项目架构图：

![3523](https://wx3.sinaimg.cn/large/006nJKqbly1g0qy5evyv5j31ax0o57al.jpg)



## 读取模块的实现

- 打开文件
- 从文件末尾开始逐行读取
- 写入到Read Channel

## 解析模块的实现

- 从Read Channel中读取每行日志数据
- 正则提取所需的监控数据
- 写入Write Channel

## 写入模块的实现

- 初始化influxdb client
- 从Write Channel中读取监控数据
- 构造数据并写入到influxdb

## 监控模块的实现

- 总处理日志行数
- 系统吞吐量
- read channel 长度
- write channel 长度
- 运行总时间
- 错误数



## 说明

时过境迁，在我手撸这个项目时，influxdb的golang库地址已经迁移到`https://github.com/influxdata/influxdb1-client`，故而在引入库的时候，改为：

```go
import "github.com/influxdata/influxdb1-client/v2"
```

代码可根据慕课网课程对应章节在tag里检出，它们分别为：

> lesson2-1：		    日志分析系统实战
>
> lesson2-2：		    代码优化
>
> lesson2-3：		    读取模块实现
>
> lesson2-4：		    解析模块实现
>
> lesson2-5And2-6：       写入模块实现
>
> lesson2-8：		    监控模块实现



