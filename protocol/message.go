package protocol

import (
	"fmt"
	"math/rand"
	"regexp"
)

type Config struct {
	Data   Data    `json:"settings"`
	Points []Point `json:"points"`
}

type Data struct {
	Dir        string `json:"direct"`
	Hostname   string `json:"hostname"`
	RTCPP      int    `json:"RTCPP"`
	RUDPP      int    `json:"RUDPP"`
	DataShards int    `json:"dataShards"`
	ParShards  int    `json:"parShards"`
}

type Point struct {
	Address                 string   `json:"address"`
	Alias                   string   `json:"alias"`
	Type                    []string `json:"type"`
	Size                    int      `json:"size"`
	PeriodPacketNum         int64    `json:"periodPacketNum"`
	PercentPacketIntervalMs int64    `json:"percentPacketIntervalMs"`
	PeriodNum               int64    `json:"periodNum"`
	PeriodMs                int64    `json:"periodMs"`
}
type Message struct {
	Address                 string
	Endpoint                string
	Size                    int
	Sequence                uint64
	Protocol                string
	IsFinish                bool
	Alias                   string
	PeriodPacketNum         uint64
	PercentPacketIntervalMs int64
	PercentPacketIntervalUs int64
	PeriodNum               int64
	Step                    int64
	Hostname                string
	PeriodMs                uint64
	PacketSeq               int
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// 提取tcp和udp数据包编号
var (
	reg = regexp.MustCompile(`[\d]+`)
)

// 需要优化
func randSeq(message *Message) string {
	seq := fmt.Sprintf("%v", message.Sequence)
	b := make([]rune, message.Size-len(seq))
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b) + seq
}

//func fillRandData(fillLength int, msg []byte) {
//	b := make([]byte, fillLength)
//	rand.Read(b)
//	for i := transmitter.EchoReplyIPHeaderLength + transmitter.EchoRequestType; i < len(msg); i++ {
//		msg[i] = b[i-transmitter.EchoRequestType-transmitter.EchoReplyIPHeaderLength]
//	}
//}
