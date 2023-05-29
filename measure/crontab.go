package measure

import (
	"Traceroute/protocol"
	"Traceroute/state"
	"Traceroute/statisticsAnalyse"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

const waitLastPacketTimeMs = 500

func startRecvGoRoutine(message *protocol.Message, conn net.Conn, recv *sync.Map) {
	//go recvICMPPacket(message, conn, recv)
	go protocol.RecvICMPPacket(message, conn, recv)
}

func task(sendPacket func(*protocol.Message, net.Conn, *sync.Map), message *protocol.Message, conn net.Conn, recv *sync.Map) (cells []statisticsAnalyse.Cell, res []float64, sequences []uint64) {
	var packets uint64
	var once sync.Once
	start := time.Now()
	duration := time.Duration(message.PercentPacketIntervalMs)*time.Millisecond + time.Duration(message.PercentPacketIntervalUs)*time.Microsecond - time.Duration(180)*time.Nanosecond*1000 //180微妙的goroutine切换
	message.IsFinish = false
	once.Do(func() { //开启接收线程
		//go recvICMPPacket(message, conn, recv)
		go startRecvGoRoutine(message, conn, recv)
	})
	for {
		//select {
		//case <-ticker.C:
		if message.PeriodPacketNum == packets { // 一个周期的测量结束。先判断，再发包，发包调用在后面
			realT := uint64(time.Since(start))
			basT := message.PeriodPacketNum * uint64(message.PercentPacketIntervalMs*1000+message.PercentPacketIntervalUs) * uint64(time.Microsecond)
			if realT > basT {
				//logger.Debug(fmt.Sprintf("发送%v个包总时间=%vns，理论时间=%vns,平均每包误差=%vus", message.PeriodPacketNum, realT, basT, (realT-basT)/message.PeriodPacketNum/1e3))
				logger.Debug(fmt.Sprintf("send\tsend%vpackets,total time=%vns,Theoretical time=%vns,Average error per packet=%vus\tcpu:%v,mem:%v", message.PeriodPacketNum, realT, basT, (realT-basT)/message.PeriodPacketNum/1e3, state.LogCPU, state.LogMEM))
			} else {
				//logger.Debug(fmt.Sprintf("发送%v个包总时间=%vns，理论时间=%vns,平均每包误差=%vus", message.PeriodPacketNum, realT, basT, (basT-realT)/message.PeriodPacketNum/1e3))
				logger.Debug(fmt.Sprintf("send\tsend%vpackets,total time=%vns,Theoretical time=%vns,Average error per packet=%vus\tcpu:%v,mem:%v", message.PeriodPacketNum, realT, basT, (realT-basT)/message.PeriodPacketNum/1e3, state.LogCPU, state.LogMEM))
			}
			//发完了所有包，就等待
			for i := 0; i < waitLastPacketTimeMs; i++ {
				// 获取最后一个包的信息
				value, ok := recv.Load(message.PeriodPacketNum)
				if !ok {
					//logger.Error("key is not exist")
					logger.Error(fmt.Sprintf("receive\tconfig is not exist\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
					return nil, nil, nil
				}
				rec, ok := value.(statisticsAnalyse.RecvStatic) //类型判断
				if !ok {
					//logger.Error("value'type is not support")
					logger.Error(fmt.Sprintf("receive\ttype of value is not support\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
					return nil, nil, nil
				}
				if rec.IsValid {
					//logger.Debug("received last packet ", zap.Int("time ms", i))
					logger.Debug(fmt.Sprintf("receive\treceived last packet\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
					break
				} else {
					time.Sleep(time.Millisecond)
				}
				if i == waitLastPacketTimeMs-1 {
					//logger.Debug("wait packet timeout", zap.Uint64("time ms", waitLastPacketTimeMs))
					logger.Debug(fmt.Sprintf("receive\twait packet timeout\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
				}
			}
			// 关闭管道本次接收线程关闭
			message.IsFinish = true
			sequences = make([]uint64, message.PeriodPacketNum)
			validNum := 0
			rtt := make([]float64, message.PeriodPacketNum)
			// 遍历所有sync.Map中的键值对
			recv.Range(func(key, value interface{}) bool {
				res, ok := value.(statisticsAnalyse.RecvStatic)
				if !ok {
					//logger.Error("error value type ,it must be config.RecvStatic", zap.Any("type", value))
					return true
				}
				seq, ok := key.(uint64)
				if !ok {
					//logger.Error("error value type ,it must be uint64", zap.Any("type", value))
					return true
				}
				if res.IsValid {
					//res.RTT = float64(res.RecvTimeStamp - res.SendTimeStamp)
					//recv.Store(seq, res)
					sequences[validNum] = seq
					validNum++
				}
				return true
			})
			if validNum == 0 { // 判断本次测量是否为有效测量
				//logger.Error("validPacketNum = 0, please check your network:", zap.String("message.Protocol", message.Protocol), zap.String("message.Address", message.Address))
				logger.Error(fmt.Sprintf("receive\tvalidPacketNum = 0, please check your network\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
				//fmt.Println(fmt.Sprintf("receive\tvalidPacketNum = 0, please check your network\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
				res := returnRes()
				//res := returnRes()
				cells = ConvertResultToCells(message, res)
				return cells, res, nil
			}
			sort.Slice(sequences, func(i, j int) bool { //对获取的有效数据下标进行排序
				return sequences[i] <= sequences[j]
			})
			zeroLocation := 0 // 定位值为0的下标的位置，正确seq 1-n...
			//??
			for i := range sequences {
				if sequences[i] != 0 {
					zeroLocation = i // 删除值为0的下标及数据
					break
				}
			}
			sequences = sequences[zeroLocation:] // 获取到有效的seq
			//得到有回显的包的rtt
			for i, seq := range sequences {
				if seq != 0 {
					value, ok := recv.Load(seq)
					if !ok {
						//logger.Error("can'realT find seq", zap.Uint64("seq", seq))
						logger.Error(fmt.Sprintf("receive\tcan't find real seq\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
						continue
					}
					res, ok := value.(statisticsAnalyse.RecvStatic)
					if !ok {
						//logger.Error("error value type,it must be config.RecvStatic", zap.Any("res", res))
						logger.Error(fmt.Sprintf("receive\terror value type,it must be config.RecvStatic\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
						continue
					}
					rtt[i] = res.RTT
					//						logger.Debugf("seq = %v, statis = %v", seq, res)
				}
			}
			// 统计值的切片
			res = statisticsAnalyse.GetResult(sequences, recv)
			// 进行数据封装
			cells = ConvertResultToCells(message, res) // 获取每个值的结果共16项
			return cells, res, sequences
		}
		sendPacket(message, conn, recv)
		packets++
		time.Sleep(duration)
	}
	return nil, nil, nil
}

func ConvertResultToCells(message *protocol.Message, res []float64) []statisticsAnalyse.Cell {
	//res := returnRes()
	typeOfData := make([]string, len(res))
	typeOfDataOrm := make([]string, len(res))
	initDataType(typeOfData, typeOfDataOrm)           //初始化数据类型
	cells := make([]statisticsAnalyse.Cell, len(res)) //后序给 icmp ttl预留一个位置
	//格林威治时间1970年01月01日00时00分00秒起到此时此刻的格林威治时间（假如是00点）的【总秒数
	ts := time.Now().Unix()
	//返回将s中前n个不重叠old子串都替换为new的新字符串，如果n<0会替换所有old子串。
	message.Alias = strings.Replace(message.Alias, "_", ".", -1)
	//packetSize := message.Size
	for i := 0; i < len(cells); i++ {
		cells[i].Step = message.Step
		cells[i].Endpoint = message.Endpoint
		cells[i].SourceName = message.Hostname
		cells[i].Value = res[i]
		cells[i].Timestamp = ts
		//cells[i].CounterType = "GAUGE"
		cells[i].Tags = fmt.Sprintf("detail="+typeOfDataOrm[i]+",packet_size=%d,num=%d,interval=%dms_%dus,p_Num=%d,p_Interval=%d,p_seq=%d,ts=%d", message.Size, message.PeriodPacketNum, message.PercentPacketIntervalMs, message.PercentPacketIntervalUs, message.PeriodNum, message.PeriodMs, message.PacketSeq, ts)
		//cells[i].Tags = fmt.Sprintf("packet_size=%d,num=%d,interval=%dms_%dus,ts=%d", message.Size, message.PeriodPacketNum, message.PercentPacketIntervalMs, message.PercentPacketIntervalUs, ts)
		cells[i].Metric = "proc." + message.Protocol + "." + message.Alias + typeOfData[i]
	}
	return cells

}

func generateFailMeasureCells(message *protocol.Message, res []float64) []statisticsAnalyse.Cell {
	return ConvertResultToCells(message, res)
}
func initDataType(typeOfData []string, typeOfDataOrm []string) {
	for i := 0; i < len(typeOfData); i++ {
		switch i {
		case 0:
			typeOfDataOrm[i] = "rtt_avg"
			typeOfData[i] = ".rtt.avg"
		case 1:
			typeOfDataOrm[i] = "rtt_var"
			typeOfData[i] = ".rtt.var" //方差
		case 2:
			typeOfDataOrm[i] = "rtt_min"
			typeOfData[i] = ".rtt.min"
		case 3:
			typeOfDataOrm[i] = "rtt_max"
			typeOfData[i] = ".rtt.max"
		case 4:
			typeOfDataOrm[i] = "rtt_quantile25"
			typeOfData[i] = ".rtt.quantile25"
		case 5:
			typeOfDataOrm[i] = "rtt_quantile50"
			typeOfData[i] = ".rtt.quantile50"
		case 6:
			typeOfDataOrm[i] = "rtt_quantile75"
			typeOfData[i] = ".rtt.quantile75"
		case 7:
			typeOfDataOrm[i] = "rtt_jitter_avg"
			typeOfData[i] = ".rtt.jitter.avg"
		case 8:
			typeOfDataOrm[i] = "rtt_jitter_var"
			typeOfData[i] = ".rtt.jitter.var"
		case 9:
			typeOfDataOrm[i] = "rtt_jitter_min"
			typeOfData[i] = ".rtt.jitter.min"
		case 10:
			typeOfDataOrm[i] = "rtt_jitter_max"
			typeOfData[i] = ".rtt.jitter.max"
		case 11:
			typeOfDataOrm[i] = "rtt_jitter_quantile25"
			typeOfData[i] = ".rtt.jitter.quantile25"
		case 12:
			typeOfDataOrm[i] = "rtt_jitter_quantile50"
			typeOfData[i] = ".rtt.jitter.quantile50"
		case 13:
			typeOfDataOrm[i] = "rtt_jitter_quantile75"
			typeOfData[i] = ".rtt.jitter.quantile75"
		case 14:
			typeOfDataOrm[i] = "packet_loss"
			typeOfData[i] = ".packet.loss"
		case 15:
			typeOfDataOrm[i] = "ttl_avg"
			typeOfData[i] = ".ttl.avg"
		case 16:
			typeOfDataOrm[i] = "ttl_var"
			typeOfData[i] = ".ttl.var"
		case 17:
			typeOfDataOrm[i] = "ttl_min"
			typeOfData[i] = ".ttl.min"
		case 18:
			typeOfDataOrm[i] = "ttl_max"
			typeOfData[i] = ".ttl.max"
		case 19:
			typeOfDataOrm[i] = "ttl_quantile25"
			typeOfData[i] = ".ttl.quantile25"
		case 20:
			typeOfDataOrm[i] = "ttl_quantile50"
			typeOfData[i] = ".ttl.quantile50"
		case 21:
			typeOfDataOrm[i] = "ttl_quantile75"
			typeOfData[i] = ".ttl.quantile75"
		}
	}
}
