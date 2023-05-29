package protocol

import (
	"Traceroute/state"
	"fmt"
	"strings"
	"time"
)

//打日志
func sendFeedback(message *Message, proto string, duration int64, ttl int, seq string) {
	var name string
	if message.Alias != "" {
		name = message.Alias
	} else {
		name = message.Address
	}
	if strings.EqualFold(seq, "icmp") {
		seq = fmt.Sprintf("%v", message.Sequence)
	}
	if strings.EqualFold(seq, "") {
		seq = fmt.Sprintf("%v", -1)
	}
	if duration != -1 {
		logger.Debug(fmt.Sprintf("receive\treceive message%s,%s,%s,%d,%d,%d,%d\tcpu:%v,mem:%v", seq, name, proto, message.Size, duration, ttl, time.Now().UnixNano(), state.LogCPU, state.LogMEM))
	} else {
		logger.Debug(fmt.Sprintf("receive\treceive message%s,%s,%s,%d,%d,%d,%d %s\tcpu:%v,mem:%v", seq, name, proto, message.Size, duration, ttl, time.Now().UnixNano(), " PacketLoss", state.LogCPU, state.LogMEM))
	}
}
