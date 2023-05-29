package measure

import (
	"Traceroute/config"
	"Traceroute/dataStore/dao"
	"Traceroute/mylog"
	"Traceroute/n9e"
	"Traceroute/protocol"
	"Traceroute/state"
	"Traceroute/statisticsAnalyse"
	"fmt"
	"go.uber.org/zap"
	"math"
	"math/rand"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

var logger = mylog.GetLogger()
var TsFlag int64
var Prepare int64
var Test bool
var TTLInfo = make(map[string]int)

func StartPing(conf *config.Config, confChan chan config.Config) {
	message := make([]protocol.Message, len(conf.Points)+1) //用来保存每个节点的测量配置
	cells := make([]statisticsAnalyse.Cell, 0)              //用来保护统计数据
	//var timePushStart time.Time
	var lock sync.Mutex
	res := returnRes()
	//res := make([]float64, 15)
	// 测量失败将值置为-9
	//for i := range res {
	//	res[i] = -9
	//}
	curTime := time.Now().UnixMilli()
	TsFlag = curTime
	// 顺序打乱 降低流量汇集概率，
	/*zm
	seed 只用于决定一个确定的随机序列。不管seed多大多小，只要随机序列一确定，本身就不会再重复。除非是样本空间太小。解决方案有两种：

	在全局初始化调用一次seed即可
	每次使用纳秒级别的种子（强烈不推荐这种）在高并发下，即使使用UnixNano作为解决方案，同样会得到相同的时间戳，Go官方也不建议在服务中同时调用。
	crypto/rand是为了提供更好的随机性满足密码对随机数的要求，
	在linux上已经有一个实现就是/dev/urandom，
	crypto/rand 就是从这个地方读“真随机”数字返回，但性能比较慢,相差10倍左右
	*/
	rand.Seed(time.Now().UnixNano())
	if conf.Data.Mixture {
		lock.Lock()
		// 洗牌算法，生成随机不重复的序列
		perm := rand.Perm(len(conf.Points))
		points := make([]config.Point, len(conf.Points)+1) //临时变量
		for i, p := range perm {
			points[i] = conf.Points[p]
		}
		points[len(points)-1] = conf.Points[0]
		points[len(points)-1].Alias = "BJ_DB"
		points[len(points)-1].Address = strings.Split(conf.Data.MysqlAddress, ":")[0]
		points[len(points)-1].Size = 1024
		conf.Points = points //把顺序写回
		lock.Unlock()
	}
	Prepare = time.Now().UnixNano() - Prepare
	for i := 0; i < len(message); i++ {
		//if conf.Data.IsContinuity {//目前它永远都是false
		//	timePushStart = time.Now()
		//}
		message[i].Address = conf.Points[i].Address
		message[i].Endpoint = conf.Data.Hostname
		message[i].Size = conf.Points[i].Size
		message[i].Sequence = 1
		message[i].Protocol = conf.Points[i].Type
		message[i].Step = conf.Data.Step
		message[i].PeriodPacketNum = conf.Points[i].PeriodPacketNum
		message[i].Alias = conf.Points[i].Alias
		message[i].PercentPacketIntervalMs = conf.Points[i].PerPacketIntervalMs // 包间隔 发送超时设定
		message[i].PercentPacketIntervalUs = conf.Points[i].PerPacketIntervalUs // 包间隔 发送超时设定
		message[i].Hostname = conf.Data.Hostname
		message[i].PeriodMs = conf.Points[i].PeriodMs
		message[i].PeriodNum = conf.Points[i].PeriodNum
		message[i].PacketSeq = i

		//var measureFun func(*protocol.Message, net.Conn, *sync.Map)
		var measureFun func(*protocol.Message, net.Conn, *sync.Map)
		//var wg sync.WaitGroup
		var conn net.Conn
		var err error
		conn, err = net.DialTimeout("ip4:icmp", message[i].Address, time.Second) //超时时间为1s
		// icmp测量失败 后序该点测量直接略过 设为-9
		if err != nil {
			//logger.Error(conf.Data.Hostname+"_to_"+message[i].Alias+" network connect fail:", zap.Error(err))
			logger.Error(fmt.Sprintf("send\tnetwork connect fail,hostname %v to %v\tcpu:%v,mem:%v", conf.Data.Hostname, message[i].Alias, state.LogCPU, state.LogMEM))
			cells = append(cells, generateFailMeasureCells(&message[i], res)...)
			//待修改
			continue
		}
		measureFun = protocol.SendICMPPacket
		// 发包 如果size配为0则发送一次64再发送一次1024
		var recv *sync.Map
		for k := int64(0); k < message[i].PeriodNum; k++ { //每个点->每个协议->每个周期
			//一般PeriodNum都为1，所以其实只发了一次
			recv = nil
			recv = new(sync.Map)
			//startTest := time.Now()
			//待修改
			//logger.Debug("init message info end, start sending ...", zap.String("time", time.Since(startTest).String()))
			logger.Debug(fmt.Sprintf("send\tinit message info end, start sending ...\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			//真正发包
			cells1, _, _ := task(measureFun, &message[i], conn, recv) //分别传入测量函数、测量节点配置、链接、包序号map
			//logger.Debug("end sending, ...", zap.String("init", time.Since(startTest).String()))
			logger.Debug(fmt.Sprintf("send\tend sending ...\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			//每次测量需要的暂停时间，即发包间隔
			time.Sleep(time.Duration(message[i].PeriodMs) * time.Millisecond)
			// 上传到openfalcon的统计值
			for n := range cells1 {
				cells1[n].SourceIp = conf.Data.MyPublicIp
				cells1[n].DestIp = message[i].Address
			}
			cells = append(cells, cells1...)
			//fmt.Println(len(cells1), cells1)
			ttlAvg := cells1[15].Value
			ttlStd := math.Sqrt(cells1[16].Value)
			TTLInfo[message[i].Address] = int(math.Floor(ttlAvg + 3*ttlStd + 1))
			if cells1 == nil || cells1[0].Value == -9 || cells1[15].Value <= 0 {
				//logger.Error("measure fail", zap.String("hostname", message[i].Alias), zap.String("protocol", message[i].Protocol), zap.String("addr", message[i].Address), zap.Int("size", message[i].Size))
				logger.Error(fmt.Sprintf("send\tmeasure fail. hostname:%v,addr:%v\tcpu:%v,mem:%v", message[i].Alias, message[i].Protocol, state.LogCPU, state.LogMEM))
				continue
			} else if message[i].Size == 1024 {
				logger.Debug(fmt.Sprintf("send\tmeasure success. hostname:%v,addr:%v\tcpu:%v,mem:%v", message[i].Alias, message[i].Protocol, state.LogCPU, state.LogMEM))
			}
			//logger.Debug("runtime.NumGoroutine()", zap.Int("num", runtime.NumGoroutine()))
			//logger.Info("发送情况", zap.String("hostname", message[i].Alias), zap.Int("size", message[i].Size), zap.Int("j", j))
			//if runtime.GOOS == "windows" {
			//logger.Debug("start to perData save to  file ...", zap.String("time", time.Since(startTest).String()))
			logger.Debug(fmt.Sprintf("store\tstart to save perData to file\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			// 存储每个探测包到本地
			//storeToFile.SavePerPacketToLocal(recv, conf, i) // 将得数据写入文件
			logger.Debug(fmt.Sprintf("store\tend save\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			recv = nil
			cells1 = nil
			message[i].Sequence = 1
		}
		if i == len(message)-1 { //是否开启循环测量的处理，相等时为测完所有节点
			//startTest := time.Now()
			cells = deleteNull(cells) // 去除空值
			//logger.Debug("marshal result 	starting ...", zap.String("time", time.Since(startTest).String()))
			logger.Debug(fmt.Sprintf("receive\tmarshal result starting ...\tstarting ...\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			for i := range cells {
				cells[i].Timestamp = curTime
			}
			//logger.Debug("end marshal ...", zap.String("time", time.Since(startTest).String()))
			logger.Debug(fmt.Sprintf("receive\tend marshal ...\tstarting ...\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			//传入数据库
			//pushToDB(cells, conf)
			//for i, _ := range cells {
			//	//返回子串str在字符串s中第一次出现的位置。
			//	//如果找不到则返回-1；如果str为空，则返回0
			//	index := strings.Index(cells[i].Tags, ",p_seq=")
			//	cells[i].Tags = cells[i].Tags[:index]
			//}
			//pushData, err := json.Marshal(cells)
			//if err != nil {
			//	logger.Error("wrong in transfer to json", zap.Error(err))
			//}
			//cells = nil
			lenConfChan := len(confChan)
			// 更新配置
			if len(confChan) != 0 {
				for l := 0; l < lenConfChan-1; l++ {
					<-confChan
				}
				newConf := <-confChan
				lock.Lock()
				*conf = newConf
				lock.Unlock()
				//logger.Info("update config ", zap.Any("conf", conf))
				logger.Info(fmt.Sprintf("manage\tupdate config\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			}
			// 连续模式
			if conf.Data.IsContinuity {
				////pushToFalcon(pushData) //直接push到服务器
				//timeStepSecond := time.Duration(conf.Data.Step-2)*time.Second - time.Since(timePushStart)
				//logger.Debug("timeStepSecond ", zap.Any("timeStepSecond", timeStepSecond))
				//i = -1
				//// 严格保证step
				//if timeStepSecond > 0 {
				//	//time.Sleep(timeStepSecond)
				//}
				////getTCPConn(conf, tcpConn)
			} else {
				// 如果模式改变，需要改变监听方式 根据监听的端口是否被占用作为标准，即使模式切换仍能保证端口有能力回显数据
				//logger.Debug("output result to terminal starting ...", zap.String("time", time.Since(startTest).String()))
				//logger.Debug("output result to terminal starting ...", zap.String("time", time.Since(startTest).String()))
				//listening.ServerListen(conf)
				// 打印到终端，openfalcon自己收集
				//str := string(pushData)
				//fmt.Println(str)
				//fmt.Println(string(pushData))
				//logger.Debug("end output ...", zap.String("time", time.Since(startTest).String()))
			}
			//wg.Wait()
		}
	}
	pushToDB(cells, conf) //crontab输出到mysql数据库必须要带的代码，但是输出到夜莺tsdb不需要
	//bytes, _ := json.Marshal(TTLInfo)
	//logger.Info("Ping Finish...", zap.String("max ttl", string(bytes)))

	//cellN9es := statisticsAnalyse.Cells2CellN9es(cells)
	err := n9e.PushDataV3(cells)
	//dao.Detail += "PushN9eFailTimes:" + strconv.Itoa(retry) + ";"
	//dao.Detail += "PushN9eT1:" + strconv.FormatInt(t1, 10) + ";"
	//dao.Detail += "PushN9eT2:" + strconv.FormatInt(t2, 10) + ";"
	if err != nil {
		dao.Detail += "PushN9eErr:" + err.Error() + ";"
		logger.Error("PushDataV3 ...", zap.String("fail", err.Error()))
	}
	////storeToFile.SavePerPacketToLocal(recv, conf, i) // 将得数据写入文件//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//for i, _ := range cells {//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//	//返回子串str在字符串s中第一次出现的位置。//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//	//如果找不到则返回-1；如果str为空，则返回0//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//	index := strings.Index(cells[i].Tags, ",p_seq=")//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//	cells[i].Tags = cells[i].Tags[:index]//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//}//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//pushData, err := json.Marshal(cells)//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//if err != nil {//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//	//logger.Error("wrong in transfer to json", zap.Error(err))//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//	//logger.Error("wrong in transfer to json", zap.Error(err))//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//}//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//fmt.Println(string(pushData))//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
	//cells = nil//夜莺插件输出到tsdb必须要带的代码，但是crontab可以不用
}
func deleteNull(cells []statisticsAnalyse.Cell) []statisticsAnalyse.Cell {
	counter := 0
	for _, cell := range cells {
		if reflect.DeepEqual(cell.Tags, statisticsAnalyse.N9e5Tags{}) || !strings.EqualFold("", cell.Tags) {
			counter++
		}
	}
	cellsCopy := make([]statisticsAnalyse.Cell, counter)
	for i, cell := range cells {
		if reflect.DeepEqual(cell.Tags, statisticsAnalyse.N9e5Tags{}) || !strings.EqualFold("", cell.Tags) {
			cellsCopy[i] = cell
		}
	}
	return cellsCopy
}
func returnRes() []float64 {
	res := make([]float64, 22)
	for i := range res {
		res[i] = -9
	}
	return res
}
func pushToDB(cells []statisticsAnalyse.Cell, conf *config.Config) {
	dao.Store(cells, conf)
}
