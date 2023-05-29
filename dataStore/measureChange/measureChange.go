package measureChange

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Data struct {
	DataModel `xorm:"extends"`

	Interval   string `gorm:"column:interval1" xorm:"interval1"`
	Num        string `gorm:"column:num" xorm:"num"`
	PInterval  string `gorm:"column:p_interval"`
	PNum       string `gorm:"column:p_num"`
	Step       string `gorm:"column:step"`
	Protocol   string `gorm:"column:protocol"`
	PacketSize string `gorm:"column:packet_size" xorm:"unique(uniquename)"`
	Value      string `gorm:"column:value"`
	// packet_loss
	// rtt_avg,rtt_min,rtt_max,rtt_var,rtt_quantile25,rtt_quantile50,rtt_quantile75
	// rtt_jitter_avg,rtt_jitter_min,rtt_jitter_max,rtt_jitter_var,rtt_jitter_quantile25,rtt_jitter_quantile50,rtt_jitter_quantile75
	// 上述15种类型
	DataType string `gorm:"-" xorm:"-"`
	PSeq     string `gorm:"p_seq"`
	Ts       string `gorm:"ts"`
}

// rtt_20200324
func (d *Data) TableName() string {
	unix := d.DataModel.TimeStamp                 //string
	unixTime, _ := strconv.ParseInt(unix, 10, 64) //int64
	timeStr := time.Unix(unixTime, 0).Local()     //string
	tail := fmt.Sprintf("%04d%02d%02d", timeStr.Year(), timeStr.Month(), timeStr.Day())
	return fmt.Sprintf("%s_%s", d.DataType, tail)
}

// TODO 实现数据格式转换
func Convert(v *MetaValue) *Data {
	dstName := strings.Split(v.Metric, "_")
	setp := v.Step
	return &Data{
		DataModel: DataModel{
			SourceIp:   v.SourceIp,
			SourceName: v.SourceName,
			DesIp:      v.DstIp,
			DesName:    v.Metric,
			TimeStamp:  v.Timestamp,
		},
		Interval:   v.Tags["interval"],
		Num:        v.Tags["num"],
		PInterval:  v.Tags["p_Interval"],
		PNum:       v.Tags["p_Num"],
		Step:       setp,
		Protocol:   strings.Split(dstName[0], ".")[1],
		PacketSize: v.Tags["packet_size"],
		Value:      strconv.FormatFloat(v.Value.(float64), 'f', -1, 64),
		DataType:   v.Tags["detail"],
		PSeq:       v.Tags["p_seq"],
		Ts:         v.Tags["ts"],
	}
}
