package dataStore

import (
	"Traceroute/config"
	"Traceroute/dataStore/dao"
	"Traceroute/protocol"
	"Traceroute/statisticsAnalyse"
	"fmt"
	"strconv"
	"testing"
	"time"
)

func Test_TS(t *testing.T) {
	formatInt := strconv.FormatInt(time.Now().Unix(), 10)
	unixTime, _ := strconv.ParseInt(formatInt, 10, 64) //int64
	timeStr := time.Unix(unixTime, 0).Local()          //string
	tail := fmt.Sprintf("%04d%02d%02d", timeStr.Year(), timeStr.Month(), timeStr.Day())
	fmt.Println(tail)
}

func Test_Table_Name(t *testing.T) {
	conf, err := config.LoadConfig("/Users/atractylodis/Documents/go-project/Traceroute/config/MeasureAgent_trace.yml",
		"/Users/atractylodis/Documents/go-project/Traceroute/config/cfg.yaml")
	if err != nil {
		fmt.Println(err.Error())
	}
	cells := make([]statisticsAnalyse.Cell, 0)
	points := conf.Points
	message := make([]protocol.Message, len(conf.Points)+1)
	for i := range points {
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
		cells = append(cells, statisticsAnalyse.Cell{
			Endpoint: message[i].Endpoint,
			Step:     message[i].Step,

			SourceName: message[i].Hostname,
			Value:      0,
			Timestamp:  time.Now().UnixMilli(),
			//cells[i].CounterType = "GAUGE"
			Tags: fmt.Sprintf(""),
			//cells[i].Tags = fmt.Sprintf("packet_size=%d,num=%d,interval=%dms_%dus,ts=%d", message.Size, message.PeriodPacketNum, message.PercentPacketIntervalMs, message.PercentPacketIntervalUs, ts)
			Metric: "proc." + message[i].Protocol + "." + message[i].Alias,
		})
	}
	table, _ := dao.Sep2MultiTable(cells, conf)
	fmt.Println(table)
}
