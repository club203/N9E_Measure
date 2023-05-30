package measure

import (
	"Traceroute/config"
	"Traceroute/dataStore/storeFile"
	transmitter2 "Traceroute/measure/transmitter/packetSendRecv"
	"Traceroute/protocol"
	"Traceroute/state"
	"Traceroute/traceroute"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	//msPerCNet  = 254
	//ipNumPerMs = 1

	// 当前存活的所有网段的数据。关联数据处理ProcessIcmpPacket和定时存储timerSave两个协程
	netDataMap  = new(sync.Map) // netDataMap[网段号加轮数] *NetPingData
	timeoutMs   = 2000
	timer       = make(chan string, timeoutMs*4)
	packetSize  = 0
	sendConn    *net.IPConn
	recvConn    *icmp.PacketConn
	recvPktConn *ipv4.PacketConn
	traceChan   = make(chan traceData, timeoutMs*4)

	traceAllIpChan   = make(chan IPRoundCnt, 100)
	traceAllIpFinish = make(chan bool, 100)

	// for data transfer
	host              = "null_host"
	measurementPrefix = "Traceroute"
)

var cfg *config.Config

type traceData struct {
	ips    []string
	sendTs int64
	round  int
}

type IPRoundCnt struct {
	ip       string
	roundCnt int
	ttl      int
}

func initTracer(nTracer int) {
	// 总共开启nTracer个协程一起发包，但要先按照10个一组，均匀分布在2秒Timeout之内。
	c := 10
	nGroup := nTracer / c
	cntTracer := 0
	var groupDelay time.Duration
	if nGroup < 1 {
		groupDelay = time.Millisecond * 10
	} else {
		groupDelay = time.Duration(traceroute.DefaultTracer.Timeout.Milliseconds()/int64(nGroup)) * time.Millisecond
	}
	for i := 0; i < nGroup; i++ {
		for j := 0; j < c; j++ {
			go traceIP(cntTracer)
			cntTracer++
		}
		time.Sleep(groupDelay)
	}
	for ; cntTracer < nTracer; cntTracer++ {
		go traceIP(cntTracer)
	}
	logger.Info("初始化trace协程", zap.Int("协程数量", cntTracer))
}

func netsToInt(nets []string) [][]int {
	var netsInt = make([][]int, 0)
	for _, str := range nets {
		splits := strings.Split(str, ".")
		tmp := make([]int, 3)
		tmp[0], _ = strconv.Atoi(splits[0])
		tmp[1], _ = strconv.Atoi(splits[1])
		tmp[2], _ = strconv.Atoi(splits[2])
		netsInt = append(netsInt, tmp)
	}
	return netsInt
}

func netCompare(net1, net2 []int) int {
	for i, num1 := range net1 {
		num2 := net2[i]
		if num1 > num2 {
			return 1
		} else if num1 < num2 {
			return -1
		}
	}
	return 0
}

func traceIP(goId int) {
	for {
		v := <-traceAllIpChan
		roundCnt := v.roundCnt
		ip := v.ip
		//var hostId = 1
		//netDataMap.Store(netId, netPingData)
		startTimestamp := time.Now().UnixMilli()
		var resultMap map[string][]string
		resultMap = map[string][]string{}
		start := time.Now()
		// trace这个IP
		oneIPStart := time.Now()
		var output []*traceroute.Hop
		var err error
		if cfg.TraceInfo.DynamicMaxTTL {
			traceCfg := traceroute.Config{MaxHops: v.ttl}
			output, err = traceroute.Trace(net.ParseIP(ip), traceCfg)
			logger.Info("", zap.String("max hops", strconv.Itoa(traceCfg.MaxHops)))
		} else {
			output, err = traceroute.TraceWithDefaultTTL(net.ParseIP(ip))
		}
		if err != nil {
			logger.Error("trace", zap.Error(err))
		}
		logger.Info("Tracing", zap.String("ip", ip), zap.String("ttl", strconv.Itoa(v.ttl)))
		length := len(output)
		if length != 0 {
			// 格式处理
			hop := output[length-1]
			result := make([]string, hop.Distance)
			for i := 0; i < len(result); i++ {
				result[i] = strconv.Itoa(i+1) + ":*"
			}
			for _, h := range output {
				for _, n := range h.Nodes {
					rtt := 0.0
					for _, r := range n.RTT {
						rtt += float64(r.Nanoseconds()) / 1e6
					}
					rtt = rtt / float64(len(n.RTT))
					// "1:148.153.93.9:-1:1.2"
					result[h.Distance-1] = strconv.Itoa(h.Distance) + ":" + n.IP.String() + ":" + "-1" + ":" +
						fmt.Sprintf("%.2f", rtt)
					//strconv.FormatFloat(rtt, 'E', -1, 64)
					//fmt.Printf("%d. %v %v\n", h.Distance, n.IP, n.RTT)
				}
			}
			// 添加到map
			resultMap[ip] = result
		}
		// 严格按照traceroute.DefaultTracer.Timeout来进行trace。防止流量汇聚。
		time.Sleep(oneIPStart.Add(traceroute.DefaultTracer.Timeout).Sub(time.Now()))

		logger.Info("TraceRoute", zap.String("ip", ip),
			zap.Float64("耗时 ms", float64(time.Since(start))/1e6),
			zap.Int("GoId", goId))

		if len(resultMap) != 0 {
			tmp, err := json.Marshal(resultMap)
			if err != nil {
				panic(err)
			}
			var traceResult = string(tmp)
			hopCount := len(resultMap[ip])
			traceResult = ip + ":" + strconv.Itoa(hopCount) + "\t" + traceResult + "\t" + fmt.Sprintf("%d", startTimestamp) + "\n"
			//logger.Debug(netId + ".0 TraceRoute done.")
			// 可以优化成只有管道+单个协程
			//todo 装配数据对象
			//hopCount := len(resultMap[ip])
			//todo 写数据库
			storeFile.SaveTraceData(ip, traceResult, roundCnt)
		}
	}
}

func initSendConn() {
	tmp, err := net.ListenIP("ip4:icmp", nil)

	sendConn = tmp
	//sendConn, err := net.DialTimeout(, mes.Address, time.Millisecond*1000)
	if err != nil {
		logger.Error("获取connection失败", zap.Any("err", err))
		pid := os.Getpid()
		cmd := fmt.Sprintf("lsof -p %d | wc", pid)
		logger.Error(cmd + ": " + transmitter2.ExecShell(cmd))
		cmd = fmt.Sprintf("lsof -p %d | grep raw | wc", pid)
		logger.Error(cmd + ": " + transmitter2.ExecShell(cmd))
		cmd = fmt.Sprintf("lsof -p %d | grep pipe | wc", pid)
		logger.Error(cmd + ": " + transmitter2.ExecShell(cmd))
	}
	raw, err := sendConn.SyscallConn()
	if err != nil {
		sendConn.Close()
		logger.Error("获取SyscallConn失败", zap.Any("err", err))
	}
	_ = raw.Control(func(fd uintptr) {
		err = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1)
	})
	if err != nil {
		sendConn.Close()
		logger.Error(fmt.Sprintf("SetsockoptInt失败"), zap.Any("err", err))
	}
}

func initRecvConn() {
	var err error
	recvConn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		logger.Error("listen err, %s", zap.Error(err))
		return
	}
	recvPktConn = recvConn.IPv4PacketConn()
	f := new(ipv4.ICMPFilter)
	f.SetAll(true)                                // 开启过滤
	f.Accept(ipv4.ICMPTypeEchoReply)              // Echo Reply
	f.Accept(ipv4.ICMPTypeDestinationUnreachable) // Destination Unreachable
	f.Accept(ipv4.ICMPTypeRedirect)               // Redirect
	f.Accept(ipv4.ICMPTypeRouterAdvertisement)    // Router Advertisement
	f.Accept(ipv4.ICMPTypeTimeExceeded)           // Time Exceeded
	f.Block(ipv4.ICMPTypeEcho)
	recvPktConn.SetControlMessage(ipv4.FlagSrc, true)
	recvPktConn.SetControlMessage(ipv4.FlagTTL, true)
	if err := recvPktConn.SetICMPFilter(f); err != nil {
		logger.Error("SetICMPFilter出错", zap.Error(err))
		os.Exit(-1)
	}
}

func StartTrace(conf *config.Config) {
	//var cfgErr error
	//cfg, cfgErr = config.LoadConfig(measureFile, configFilename)
	//if cfgErr != nil {
	//	//todo
	//}
	cfg = conf
	// 初始化发包connect
	initSendConn()
	initRecvConn()
	defer recvConn.Close()
	defer recvPktConn.Close()
	defer sendConn.Close()

	//traceRoutineNum := 250
	// 开启定时存储
	go timerSave()
	// 开启TraceRoute协程
	//for i := 0; i < traceRoutineNum; i++ {
	//	go Traceroute(i)
	//}
	initTracer(1)

	// 开启数据包接收
	recvRoutineNum := 10
	for i := 0; i < recvRoutineNum; i++ {
		go protocol.RecvAllIcmpPacket(netDataMap, recvPktConn, i)
	}
	durationHours := time.Duration(-1)
	roundCnt := 0
	//todo 动态变更测量配置
	//configChan := make(chan config.Config, 10)
	//config.DynamicUpdateConfig(configFilename, configChan)
	des := cfg.Points
	expectedPeriodMinutes := cfg.Setting.ExpectedPeriodMinutes
	for {
		//if !win10 {
		//	listening.ServerListen(conf) //开启服务器监听
		//}
		start := time.Now().Unix()
		logger.Info("测量开始", zap.Int("轮数", roundCnt))
		confChan := make(chan config.Config, 10) //带缓存的channel，无缓存的channel的读写都将进行堵塞
		//config.DynamicUpdateConfig(configurationFilename, cfgFilename, confChan) //Linux赋权限和更新配置
		//monitorProc()
		//logger.Debug("init end, wait server starting ...", zap.String("time", time.Since(start).String()))
		logger.Debug(fmt.Sprintf("manage\tinit end, wait to server starting ...\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		//listening.WaitServer(&conf) //等待其他节点启动
		//logger.Debug("wait server end,start measuring ...", zap.String("time", time.Since(start).String()))
		logger.Debug(fmt.Sprintf("manage\tstart measuring ...\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		//开启ping测量
		StartPing(conf, confChan)
		//todo 动态变更测量配置
		//select {
		//case cfg = <-configChan:
		//default:
		//}
		//if config.CfgMeansStop(cfg) {
		//	return
		//}
		host = cfg.Setting.Hostname
		measurementPrefix = cfg.Setting.Label

		for _, node := range des {
			ttl := TTLInfo[node.Address]
			if ttl < 0 {
				ttl = 64
			}
			if node.Address != cfg.Data.Hostname {
				cnt := IPRoundCnt{
					ip:       node.Address,
					roundCnt: roundCnt,
					ttl:      64,
				}
				traceAllIpChan <- cnt
			}
		}
		// 针对每个优先级的区域进行探测
		//for ii := 0; ii < len(cfg.Points); ii++ {
		//	//priority := cfg.Points[ii].Levels
		//	ipNumPerMs = cfg.Points[ii].IPNumPerMs
		//	packetSize = cfg.Points[ii].Size
		//	netsInt := netsToInt(cfg.Points[ii].Address)
		//	// 探测一个区域中的所有地址段
		//	for i := 0; i < len(netsInt)/2; i++ {
		//		startNetId := netsInt[i*2]
		//		endNetId := netsInt[i*2+1]
		//		probeNet(startNetId, endNetId, roundCnt, cfg.Points[ii].EnableTrace)
		//	}
		//	//time.Sleep(time.Minute)
		//	//time.Sleep(time.Second)
		//}
		//time.Sleep(time.Duration(10) * time.Second)
		roundCnt++
		now := time.Now()
		cost := now.Unix() - start
		// 不sleep 一直trace
		if expectedPeriodMinutes != 0 {
			sleepSecond := expectedPeriodMinutes*60 - cost
			logger.Info("等待本轮测量结束", zap.String("time", strconv.FormatInt(sleepSecond, 10)))
			time.Sleep(time.Duration(sleepSecond) * time.Second)
			//continue
		}
		if durationHours == time.Duration(-1) {
			// 整小时时间还没有对齐
			hours := (time.Duration(cost) * time.Second) / time.Hour
			hours += 1
			durationHours = hours
			// 获取当前的小时对应的timestamp second
			//currentHourSec := now.Unix() - int64(now.Second()) - int64(60*now.Minute())
			// 下一个小时
			//nextHour := time.Duration(currentHourSec)*time.Second + time.Hour
			// 距离下一个小时的时间
			//timeToNextHour := nextHour - time.Duration(now.UnixNano())
			// sleep 距离下一个整点时间的timeToNextHour
			//time.Sleep(timeToNextHour)
		} else {
			// 整小时时间已经对齐
			//time.Sleep(durationHours*time.Hour - time.Duration(cost)*time.Second)
		}
	}
}

func timerSave() {
	for {
		if len(timer) == 0 {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		ip := <-timer // 这里读出来数netid_轮数 1.1.1_1
		//netId, round := GetNetIdRound(netIdRound)
		load, ok := netDataMap.Load(ip)
		if !ok {

			//  这里有可能会出错，出错原因是在一个测量超时时间内重复测量了一个C段
			//  导致重复测量的原因是配置文件中存在相同的C段且相邻
			//  可以忽略这个错误，不会对测量结果产生任何影响
			ips := make([]string, 92)
			netDataMap.Range(func(key, value interface{}) bool {
				ips = append(ips, key.(string))
				return true
			})
			logger.Error("Load出错", zap.String("ip", ip))
			logger.Error("Load出错", zap.Strings("netDataMap中所有的ip", ips))
			continue
		}
		netPingData, ok := load.(*protocol.NetPingData)
		if !ok {
			logger.Error("sync.Map load转换出错")
			continue
		}
		// 等待在map中清除这个网段的数据, 两倍IP过期时间
		timeout := time.Duration(netPingData.TimeoutMs*2) * time.Millisecond
		time.Sleep(timeout - time.Since(netPingData.SentAllPktTs))
		netDataMap.Delete(ip)
		//logger.Debug("清除", zap.String("C", netId))
		logger.Info("清除", zap.String("ip", ip))

		// 开始存储。对RttMap中的key-value排序
		keys := make([]string, 255)
		size := 0
		netPingData.RttMap.Range(func(key, value interface{}) bool {
			k, _ := key.(string)
			keys[size] = k
			size++
			return true
		})
		logger.Info("收包结束", zap.String("ip", ip))
		//没有数据存个毛
		if size == 0 {
			continue
		}

		keys = keys[:size]                     //key 是ip
		sort.Slice(keys, func(i, j int) bool { //对获取的有效数据下标进行排序
			if len(keys[i]) == len(keys[j]) {
				return keys[i] <= keys[j]
			} else {
				return len(keys[i]) <= len(keys[j])
			}
		})
		// 对这些ip进行筛选，然后traceroute。 如果开启的话
		//if netPingData.OpenTrace {
		//	data := traceData{
		//		ips:    keys,
		//		sendTs: netPingData.SentAllPktTs.Unix(),
		//		round:  netPingData.Round,
		//	}
		//select {
		//case traceChan <- data:
		//default:
		//	logger.Error("traceChan满了")
		//	go func() { traceChan <- data }()
		//}
		//go Traceroute(netId, keys, netPingData.SentAllPktTs.Unix(), netPingData.Round)
		//}

		// 对value排序
		values := make([]string, size)
		for i := 0; i < size; i++ {
			k := keys[i]
			value, _ := netPingData.RttMap.Load(k)
			v := value.(string)
			values[i] = v
		}

		// 将数据发到transfer转储
		// 吐了，这里还要再拆分
		//for _, v := range values {
		//	// v == ip + ":" + fmt.Sprintf("%d", ttl) + ":" + fmt.Sprintf("%.3f", rtt)
		//	splits := strings.Split(v, ":")
		//	dest := splits[0]
		//	rtt64, _ := strconv.ParseFloat(splits[2], 32)
		//	_, err := storeDB.SendPingData(context.TODO(), &pb.TransferPingRequest{
		//		MeasurementBaseResult: &pb.MeasurementBaseInfo{
		//			RecvTimestampMillisecond: netPingData.SentAllPktTs.UnixMilli(),
		//			Host:                     host,
		//			Destination:              dest,
		//			Protocol:                 pb.ProbeProtocol_ICMP,
		//			Round:                    int32(netPingData.Round),
		//			MeasurementPrefix:        measurementPrefix,
		//		},
		//		RttMillisecond: float32(rtt64),
		//	})
		//	if err != nil {
		//		logger.Error("storeDB.SendPingData error ", zap.Error(err))
		//	}
		//}

		tmp, err := json.Marshal(values)
		if err != nil {
			panic(err)
		}
		var rttResult = string(tmp)
		rttResult = ip + "\t" + rttResult + "\t" + fmt.Sprintf("%d", netPingData.SentAllPktTs.UnixMilli()) + "\n"
		//写入文件
		storeFile.SavePingData(ip, rttResult, netPingData.Round)
		logger.Debug("测量结果", zap.String("trace", rttResult))
	}
}
