package protocol

import (
	"Traceroute/config"
	"Traceroute/mylog"
	"Traceroute/state"
	"Traceroute/statisticsAnalyse"
	"Traceroute/traceroute"
	"encoding/binary"
	"fmt"
	"go.uber.org/zap"
	icmp_ex "golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	EchoReplyIPHeaderLength = 20
	EchoReplyType           = 0
	EchoSendFastType        = 4
	// IANA Assigned Internet Protocol Numbers
	ProtocolICMP     = 1
	ProtocolTCP      = 6
	ProtocolUDP      = 17
	ProtocolIPv6ICMP = 58

	GetIP = 12

	EchoRequestHeaderLength = 20
	EchoRequestType         = 8
	DefaultTTL              = 64
)

var (
	logger = mylog.GetLogger()
	conf   = config.GetConfig()
	pid    = os.Getpid()
)

type IpPacketT struct {
	ip            string
	recvTimestamp int64 // 微妙级别
	recvData      []byte
	length        int
	Round         int // 第几轮测量
}

type NetPingData struct {
	IP string
	//SendTimestamp map[string]int64 //每个ip的发送时间戳
	SendTimestamp *sync.Map //每个ip的发送时间戳 <string, int64>
	//Rtt           map[string]float64
	RttMap       *sync.Map //并行处理包的时候直接覆盖写  <string, float64>
	SentAllPktTs time.Time //发送完这个网段所有包时的时间
	//定时存储协程
	TimeoutMs int  //超时时间，ms
	Round     int  // 第几轮测量
	OpenTrace bool // 是否开启trace
}

// 只收包，不处理
func RecvAllIcmpPacket(netDataMap *sync.Map, conn *ipv4.PacketConn, goId int) {
	//conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	//packetConn := conn.IPv4PacketConn()
	//if err != nil {
	//	logger.Error("listen err, %s", zap.Error(err))
	//	return
	//}
	//defer conn.Close()
	for {
		//packetLength := conf.Points.Size
		packetLength := 1024
		rb := make([]byte, packetLength+20)
		// 接收出来rb[0]就是报文类型，rb[1]为code,rb[2,3]为校验和
		// rb[4,5]为ID标识符两个字节，rb[6,7]为序列号
		n, cm, peer, err := conn.ReadFrom(rb)
		if err != nil {
			logger.Error("接收出错: ", zap.Error(err))
			continue
		}
		ip := peer.String()
		//index := strings.LastIndex(ip, ".")
		//netId := ip[:index]
		//// 主机地址为255的直接丢
		//hostId := ip[index+len("."):]
		//if hostId == "255" {
		//	continue
		//}
		// 过滤过期包或者不为接收ip的包
		load, ok := netDataMap.Load(ip)
		//traceroute.DefaultTracer.ServeData(cm.Src, rb)
		if !ok || rb[0] != 0 {
			logger.Debug("Recv到不存在的包", zap.String("IP", ip),
				zap.Any("报文类型", rb[0]), zap.Any("code", rb[1]),
				zap.Int("GoId", goId))
			traceroute.DefaultTracer.ServeData(cm.Src, rb)
			continue
		}
		netPingData := load.(*NetPingData)
		ProcessIcmpPacket1R(ip, rb, n, cm.TTL, time.Now().UnixNano()/1e3, netPingData, goId)
	}
}

func ProcessIcmpPacket1R(ip string, packetData []byte, length, ttl int, recvTs int64, netPingData *NetPingData, goId int) {
	// 接收出来rb[0]就是报文类型，rb[1]为code,rb[2,3]为校验和
	// rb[4,5]为ID标识符两个字节，rb[6,7]为序列号
	// 先校验这个包是否有效， 以进程ID做标识
	id0, id1 := genIdentifier(pid & 0xffff)
	if packetData[4] != id0 || packetData[5] != id1 {
		//logger.Debug("收到非该进程包", zap.String("对端IP", ip), zap.Int("pid", pid))
		return
	}
	// 校验序列号
	seq := getSeq(packetData[6], packetData[7])
	if seq != 1 {
		logger.Warn("收到非序列号里的包", zap.Uint64("序号", seq), zap.String("ip", ip))
		return
	}
	rm, err := icmp_ex.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), packetData[:length])
	if err != nil {
		logger.Error("ParseMessage error", zap.Error(err))
		return
	}
	// 计算RTT，格式化结果，并放入map
	sendTsRaw, _ := netPingData.SendTimestamp.Load(ip) //us
	sendTs, ok := sendTsRaw.(int64)
	if !ok {
		ips := make([]string, 255)
		netPingData.SendTimestamp.Range(func(key, value interface{}) bool {
			ips = append(ips, key.(string))
			return true
		})
		// 出错实例： 	`sendTsRaw转换出错  {"IP": "66.235.131.0"}`
		// 这里出错是收到了属于这个C段但还没有发包探测的IP
		// 比如会收到66.235.131.0这个IP，但实际上发包的时候根本不会对着.0发
		logger.Error("sendTsRaw转换出错", zap.String("IP", ip))
		logger.Error("sendTsRaw转换出错", zap.Strings("SendTimestamp中所有的netIp", ips))
		return
	}
	rtt := float64(recvTs-sendTs) / 1e3 // ms
	if rtt < 0 {
		logger.Error("Rtt为负", zap.Float64("RTT", rtt), zap.String("IP", ip))
		return
	}
	if rtt > 1000 {
		logger.Info("Rtt大于1000", zap.Float64("RTT", rtt), zap.String("IP", ip),
			zap.Int("GoId", goId))
	}
	//ttl := int(packetData[8])
	// 不是echo回包
	//logger.Debug("收包 ", zap.String("IP", ip), zap.Any("ICMPType ", rm.Type))
	if rm.Type != ipv4.ICMPTypeEchoReply {
		// 不是echo回包
		logger.Error("不是echoReply ", zap.String("IP", ip), zap.Any("ICMPType ", rm.Type))
		rtt = float64(-rm.Type.Protocol())
	}
	var result = ip + ":" + fmt.Sprintf("%d", ttl) + ":" + fmt.Sprintf("%.3f", rtt)
	// 放入map
	store, loaded := netPingData.RttMap.LoadOrStore(ip, result)
	presentRtt, ok := store.(string)
	if !ok {
		logger.Error("sync.Map load转换出错", zap.String("IP", ip))
		return
	}
	if loaded {
		logger.Warn("重复收包 ", zap.String("IP", ip),
			zap.String("存在的值", presentRtt), zap.String("刚测的值", result),
			zap.Int("GoId", goId))
	}
}

func newPacket(id uint16, dst net.IP, ttl int) []byte {
	// TODO: reuse buffers...
	//msg := icmp.Message{
	//	Type: ipv4.ICMPTypeEcho,
	//	Body: &icmp.Echo{
	//		ID:  int(id),
	//		Seq: int(id),
	//	},
	//}
	//p, _ := msg.Marshal(nil)
	ip := &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TotalLen: ipv4.HeaderLen, //+ len(p),
		TOS:      16,
		ID:       int(id),
		Dst:      dst,
		Protocol: ProtocolICMP,
		TTL:      ttl,
	}
	buf, err := ip.Marshal()
	if err != nil {
		return nil
	}
	return buf
}

/**
  新的收包方式，同步发收
*/
func SendIcmpPacketSync1Conn(seq uint64, packetSize int, ip string, conn *net.IPConn, sendTimestamp *sync.Map) int64 {
	const EchoReplyIPHeaderLength = 20
	conf = config.GetConfig()
	id0, id1 := genIdentifier(os.Getpid() & 0xffff)
	//var id0, id1 byte = 11, 11
	//length := conf.Points.Size
	length := packetSize
	msg := make([]byte, length)
	fillLength := length - EchoRequestType - EchoReplyIPHeaderLength
	//	logger.Debugf("ppid = %v", os.Getpid()&0xfff)
	msg[0] = 8                // echo,第一个字节表示报文类型，8表示回显请求,0表示回显的应答
	msg[1] = 0                // code 0,ping的请求和应答，该code都为0
	msg[2] = 0                // checksum
	msg[3] = 0                // checksum
	msg[4], msg[5] = id0, id1 //identifier[0] identifier[1], ID标识符 占2字节
	//序号直接写死只有一个包,想了想还是留个余地。
	msg[6], msg[7] = genSequence(&seq) //sequence[0], sequence[1],序号占2字节
	fillRandData(fillLength, msg)
	check := checkSum(msg[0:length]) //计算检验和。
	msg[2] = byte(check >> 8)
	msg[3] = byte(check & 255)
	// 设置ip缓存，只有在这个缓存里面的才能被接收
	//ip := conn.RemoteAddr().String()

	b := newPacket(1, net.ParseIP(ip), 64)
	b = append(b, msg[0:length]...)
	sendTimeStamp := time.Now().UnixNano() / 1e3 //转化到us ,运行时这个也是第一个运行先取得这个函数的返回值
	// 写入时间戳，防止处理包的时候还没有记录
	sendTimestamp.Store(ip, sendTimeStamp)
	//n, err := conn.WriteTo(msg[0:length], &net.IPAddr{IP: net.ParseIP(ip)})
	n, err := conn.WriteToIP(b, &net.IPAddr{IP: net.ParseIP(ip)})
	//recv.Store(message.Sequence, res)   //非值传递，value为interface{}空接口，包含类型和类型的值(指针)两个字节，参见gopl P243
	if err != nil || n <= 0 {
		logger.Error(" network send fail:", zap.Error(err))
	}
	return sendTimeStamp

}

/**
  新的收包方式，同步发收
*/
func SendIcmpPacketSync(seq uint64, packetSize int, conn net.Conn, sendTimestamp *sync.Map) int64 {
	const EchoReplyIPHeaderLength = 20
	conf = config.GetConfig()
	id0, id1 := genIdentifier(os.Getpid() & 0xffff)
	//var id0, id1 byte = 11, 11
	//length := conf.Points.Size
	length := packetSize
	msg := make([]byte, length)
	fillLength := length - EchoRequestType - EchoReplyIPHeaderLength
	//	logger.Debugf("ppid = %v", os.Getpid()&0xfff)
	msg[0] = 8                // echo,第一个字节表示报文类型，8表示回显请求,0表示回显的应答
	msg[1] = 0                // code 0,ping的请求和应答，该code都为0
	msg[2] = 0                // checksum
	msg[3] = 0                // checksum
	msg[4], msg[5] = id0, id1 //identifier[0] identifier[1], ID标识符 占2字节
	//序号直接写死只有一个包,想了想还是留个余地。
	msg[6], msg[7] = genSequence(&seq) //sequence[0], sequence[1],序号占2字节
	fillRandData(fillLength, msg)
	check := checkSum(msg[0:length]) //计算检验和。
	msg[2] = byte(check >> 8)
	msg[3] = byte(check & 255)
	// 设置ip缓存，只有在这个缓存里面的才能被接收
	ip := conn.RemoteAddr().String()
	sendTimeStamp := time.Now().UnixNano() / 1e3 //转化到us ,运行时这个也是第一个运行先取得这个函数的返回值
	// 写入时间戳，防止处理包的时候还没有记录
	sendTimestamp.Store(ip, sendTimeStamp)
	n, err := conn.Write(msg[0:length]) //onn.Write方法执行之后也就发送了一条ICMP请求，同时进行计时和计次
	//recv.Store(message.Sequence, res)   //非值传递，value为interface{}空接口，包含类型和类型的值(指针)两个字节，参见gopl P243
	if err != nil || n <= 0 {
		logger.Error(" network send fail:", zap.Error(err))
	}
	return sendTimeStamp

}

func genIdentifier(id int) (byte, byte) {
	return uint8(id >> 8), uint8(id & 0xff)
}
func genSequence(v *uint64) (byte, byte) {
	ret := make([]byte, 8)
	if *v > 65000 {
		logger.Warn("seq overflow 65000, but it only effect Icmp", zap.Uint64("seq", *v))
		return 0, 0
	}
	binary.LittleEndian.PutUint64(ret, *v)
	return ret[1], ret[0]
}

func checkSum(data []byte) uint16 {
	var (
		sum    uint32
		length int = len(data)
		index  int
	)
	for length > 1 {
		sum += uint32(data[index])<<8 + uint32(data[index+1])
		index += 2
		length -= 2
	}
	if length > 0 {
		sum += uint32(data[index])
	}
	sum += sum >> 16

	return uint16(^sum)
}

func getSeq(seq1, seq2 byte) uint64 {
	seq := make([]byte, 8)
	seq[1] = seq1
	seq[0] = seq2
	seqUint64 := binary.LittleEndian.Uint64(seq)
	return seqUint64
}

func RecvICMPPacket(message *Message, conn net.Conn, recv *sync.Map) {
	//defer logger.Debug("goroutine return:", zap.String("message.Protocol", message.Protocol), zap.String("message.Address", message.Address))
	defer logger.Debug(fmt.Sprintf("receive\tgoroutine return. address:%v\tcpu:%v,mem:%v", message.Address, state.LogCPU, state.LogMEM))
	id0, id1 := genIdentifier(os.Getpid() & 0xffff) //父进程id作为唯一标识
	//var id0, id1 byte = 11, 11
	//var timeout int64 = 2000
	//在使用Go语言的net.Dial函数时，发送echo request报文时，不用考虑i前20个字节的ip头；
	// 但是在接收到echo response消息时，前20字节是ip头。后面的内容才是icmp的内容，应该与echo request的内容一致
	var receive []byte
	receive = make([]byte, message.Size+20)
	//reader := bufio.NewReader(conn)
	// 如果由于调度问题导致协程积累，定时清除
	start := time.Now()
	timeout := time.Duration(message.PercentPacketIntervalMs*int64(message.PeriodPacketNum))/1000*time.Second + time.Second*2
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	for !message.IsFinish && time.Since(start) < timeout {
		var endDuration int64 = 5
		//n, err := reader.Read(receive)
		// 表现为阻塞，实际上是非阻塞
		n, err := conn.Read(receive)

		if err != nil || n < 0 {
			if err != nil && strings.Contains(err.Error(), "timeout") {
				return
			}
			//logger.Error("icmp read error", zap.Error(err))
			logger.Error(fmt.Sprintf("receive\ticmp read error\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			continue
		}
		receive = receive[:n]
		//检验icmp是不是自己的
		if id0 == receive[EchoReplyIPHeaderLength+4] &&
			id1 == receive[EchoReplyIPHeaderLength+5] {
			seq := getSeq(receive[EchoReplyIPHeaderLength+6], receive[EchoReplyIPHeaderLength+7]) // 提取数据包编号
			//logger.Infof("%v hope remote addr = %v,recv seq = %d", time.Now(), message.Address, seq)
			seqRecv, ok := recv.Load(seq)
			if !ok {
				//logger.Error("recv[index] is nil", zap.String("message.Address", message.Address), zap.Uint64("seq", seq))
				logger.Error(fmt.Sprintf("receive\trecv[index] is nil.address:%v,seq:%v\tcpu:%v,mem:%v", message.Address, seq, state.LogCPU, state.LogMEM))
			}
			seqRes, ok := seqRecv.(statisticsAnalyse.RecvStatic)
			if !ok {
				//logger.Error("seqRes's type isn't RecvStatic")
				logger.Error(fmt.Sprintf("receive\tseqRes's type isn't RecvStatic\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			}
			//除了判断err!=nil，还有判断请求和应答的ID标识符，sequence序列码是否一致，以及ICMP是否超时（receive[ECHO_REPLY_HEAD_LEN] == 11，即ICMP报头的类型为11时表示ICMP超时）
			var ttl int
			//假设对面默认填入TTL也是64
			ttl = int(receive[8])
			// 解析收到的正确icmp包,需要去除重复的包
			if !seqRes.IsValid {
				endTime := time.Now()
				res := statisticsAnalyse.RecvStatic{
					Seq:           seq,
					Alias:         message.Alias,
					Proto:         message.Protocol,
					Size:          message.Size,
					RTT:           -1,
					TTL:           DefaultTTL - ttl,
					SendTimeStamp: seqRes.SendTimeStamp,
					RecvTimeStamp: endTime.UnixNano() / 1e3, // us
					IsValid:       true,
				}
				endDuration = res.RecvTimeStamp - res.SendTimeStamp
				if ttl == -1 {
					res.IsValid = false
				}
				res.RTT = float64(endDuration) / 1e3
				//logger.Infof("%v,seq = %v %v RTT = %v", message.Alias, seq, message.Protocol, res.RTT)
				recv.Store(seq, res)
				sendFeedback(message, message.Protocol, endDuration, ttl, fmt.Sprint(seq))
			}
		}
	}
	err := conn.Close()
	if err != nil {
		//logger.Error(" connection close fail:", zap.String("Address", message.Address), zap.String("Protocol", message.Protocol), zap.Error(err))
		logger.Error(fmt.Sprintf("manage\tconnection close fail.Address:%v\tcpu:%v,mem:%v", message.Address, state.LogCPU, state.LogMEM))
	}
}

func RecvICMPEchoRequest(measureSig *bool, conn net.Conn, ttlData *sync.Map, srcIPSet map[string]struct{}) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			logger.Error(fmt.Sprintf("connection cannot be closed, because %s", err.Error()))
		}
	}(conn)
	for *measureSig {
		var receive []byte
		receive = make([]byte, 1050)
		_, err := conn.Read(receive)
		if err != nil {
			logger.Error(fmt.Sprintf("read data error, %s", err.Error()))
		}
		ips := receive[GetIP : GetIP+4]
		srcIP := string(ips)
		_, ok := srcIPSet[srcIP]
		if EchoRequestType == receive[EchoRequestHeaderLength] && ok {
			restTTL := int(receive[8])
			ttl := DefaultTTL - restTTL
			load, exist := ttlData.Load(srcIP)
			if !exist {
				load = make([]int, 10)
			}
			ttls := load.([]int)
			ttls = append(ttls, ttl)
			ttlData.Store(srcIP, ttls)
		}
	}
	//本轮测量结束，数据写文件和数据库
}

//
//func SendICMPPacket(message *Message, conn icmp_ex.PacketConn, dst net.IPAddr, recv *sync.Map) {
//	if recv == nil {
//		//logger.Error("recv is empty:", zap.Any("recv", recv))
//		logger.Error(fmt.Sprintf("send\trecv is empty\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
//	}
//	icmp(message, conn, dst, recv)
//	message.Sequence += 1
//	if message.Sequence > message.PeriodPacketNum {
//		message.Sequence = 1 //发送完一个周期的探测包过后重新置为1，为下次发包做准备
//	}
//}

func SendICMPPacket(message *Message, conn net.Conn, recv *sync.Map) {
	if recv == nil {
		//logger.Error("recv is empty:", zap.Any("recv", recv))
		logger.Error(fmt.Sprintf("send\trecv is empty\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	}
	icmp(message, conn, recv)
	message.Sequence += 1
	if message.Sequence > message.PeriodPacketNum {
		message.Sequence = 1 //发送完一个周期的探测包过后重新置为1，为下次发包做准备
	}
}

//func icmp(message *Message, conn icmp_ex.PacketConn, dstIP net.IPAddr, recv *sync.Map) {
//	//const EchoReplyIPHeaderLength = 20
//	id0, id1 := genIdentifier(os.Getpid() & 0xffff)
//	//var id0, id1 byte = 11, 11
//	msg := make([]byte, message.Size)
//	fillLength := message.Size - EchoRequestType - EchoReplyIPHeaderLength
//	//	logger.Debugf("ppid = %v", os.Getpid()&0xfff)
//	msg[0] = 8                                      // echo,第一个字节表示报文类型，8表示回显请求,0表示回显的应答
//	msg[1] = 0                                      // code 0,ping的请求和应答，该code都为0
//	msg[2] = 0                                      // checksum
//	msg[3] = 0                                      // checksum
//	msg[4], msg[5] = id0, id1                       //identifier[0] identifier[1], ID标识符 占2字节
//	msg[6], msg[7] = genSequence(&message.Sequence) //sequence[0], sequence[1],序号占2字节
//	length := message.Size
//	fillRandData(fillLength, msg)
//	check := checkSum(msg[0:length]) //计算检验和。
//	msg[2] = byte(check >> 8)
//	msg[3] = byte(check & 255)
//
//	//conn, err := net.Dial("ip4:icmp", message.Address)
//	//conn, err := net.DialTimeout("ip:icmp", message.Address, time.Duration(timeout*1000*1000))
//	//	logger.Debugf("addr = %v,recv seq = %d", message.Address, message.Sequence)
//	startTime := time.Now()
//	res := statisticsAnalyse.RecvStatic{
//		Seq:           message.Sequence,
//		Alias:         message.Alias,
//		Proto:         message.Protocol,
//		Size:          message.Size,
//		RTT:           -1,
//		TTL:           0,
//		SendTimeStamp: startTime.UnixNano() / 1e3, //转化到us ,运行时这个也是第一个运行先取得这个函数的返回值
//		RecvTimeStamp: 0,
//		IsValid:       false,
//	}
//	//n, err := conn.Write(msg[0:length])
//	n, err := conn.WriteTo(msg[0:length], dstIP) //conn.Write方法执行之后也就发送了一条ICMP请求，同时进行计时和计次
//	recv.Store(message.Sequence, res)            //非值传递，value为interface{}空接口，包含类型和类型的值(指针)两个字节，参见gopl P243
//	if err != nil || n <= 0 {
//		//logger.Error(" network send fail:", zap.Uint64("message.Sequence", message.Sequence), zap.Error(err))
//		logger.Error(fmt.Sprintf("send\tnetwork send fail.Sequence:%v\tcpu:%v,mem:%v", message.Sequence, state.LogCPU, state.LogMEM))
//		return
//	}
//	//	logger.Infof("send time = %v", startTime.UnixNano()/1e6)
//}

func icmp(message *Message, conn net.Conn, recv *sync.Map) {
	//const EchoReplyIPHeaderLength = 20
	id0, id1 := genIdentifier(os.Getpid() & 0xffff)
	//var id0, id1 byte = 11, 11
	msg := make([]byte, message.Size)
	fillLength := message.Size - EchoRequestType - EchoReplyIPHeaderLength
	//	logger.Debugf("ppid = %v", os.Getpid()&0xfff)
	msg[0] = 8                                      // echo,第一个字节表示报文类型，8表示回显请求,0表示回显的应答
	msg[1] = 0                                      // code 0,ping的请求和应答，该code都为0
	msg[2] = 0                                      // checksum
	msg[3] = 0                                      // checksum
	msg[4], msg[5] = id0, id1                       //identifier[0] identifier[1], ID标识符 占2字节
	msg[6], msg[7] = genSequence(&message.Sequence) //sequence[0], sequence[1],序号占2字节
	length := message.Size
	fillRandData(fillLength, msg)
	check := checkSum(msg[0:length]) //计算检验和。
	msg[2] = byte(check >> 8)
	msg[3] = byte(check & 255)

	//conn, err := net.Dial("ip4:icmp", message.Address)
	//conn, err := net.DialTimeout("ip:icmp", message.Address, time.Duration(timeout*1000*1000))
	//	logger.Debugf("addr = %v,recv seq = %d", message.Address, message.Sequence)
	startTime := time.Now()
	res := statisticsAnalyse.RecvStatic{
		Seq:           message.Sequence,
		Alias:         message.Alias,
		Proto:         message.Protocol,
		Size:          message.Size,
		RTT:           -1,
		TTL:           0,
		SendTimeStamp: startTime.UnixNano() / 1e3, //转化到us ,运行时这个也是第一个运行先取得这个函数的返回值
		RecvTimeStamp: 0,
		IsValid:       false,
	}
	n, err := conn.Write(msg[0:length]) //conn.Write方法执行之后也就发送了一条ICMP请求，同时进行计时和计次
	recv.Store(message.Sequence, res)   //非值传递，value为interface{}空接口，包含类型和类型的值(指针)两个字节，参见gopl P243
	if err != nil || n <= 0 {
		//logger.Error(" network send fail:", zap.Uint64("message.Sequence", message.Sequence), zap.Error(err))
		logger.Error(fmt.Sprintf("send\tnetwork send fail.Sequence:%v\tcpu:%v,mem:%v", message.Sequence, state.LogCPU, state.LogMEM))
		return
	}
	//	logger.Infof("send time = %v", startTime.UnixNano()/1e6)
}

func fillRandData(fillLength int, msg []byte) {
	b := make([]byte, fillLength)
	rand.Read(b)
	for i := EchoReplyIPHeaderLength + EchoRequestType; i < len(msg); i++ {
		msg[i] = b[i-EchoRequestType-EchoReplyIPHeaderLength]
	}
}
