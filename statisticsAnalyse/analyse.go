package statisticsAnalyse

import (
	"Traceroute/mylog"
	"go.uber.org/zap"
	"math"
	"os"
	"strings"
	"sync"
)

var logger = mylog.GetLogger()

type Cell struct {
	Endpoint    string
	SourceName  string
	Metric      string
	Timestamp   int64
	Step        int64
	Value       float64
	CounterType string
	Tags        string
	SourceIp    string
	DestIp      string
}
type CellN9eV5 struct {
	Endpoint    string   `json:"-"`
	SourceName  string   `json:"-"`
	Metric      string   `json:"metric"`
	Timestamp   int64    `json:"timestamp"`
	Step        int64    `json:"-"`
	Value       float64  `json:"value"`
	CounterType string   `json:"-"`
	Tags        N9e5Tags `json:"tags"`
	SourceIp    string   `json:"-"`
	DestIp      string   `json:"-"`
}
type N9e5Tags struct {
	Describe string `json:"describe"`
	Ident    string `json:"ident"`
}

//每个包的记录信息
type RecvStatic struct {
	Seq           uint64
	Alias         string
	Proto         string
	Size          int
	RTT           float64
	TTL           int
	SendTimeStamp int64
	RecvTimeStamp int64
	IsValid       bool
}

func GetResult(seqs []uint64, recv *sync.Map) []float64 {
	rtts := make([]float64, len(seqs))
	ttls := make([]int, len(seqs))
	if recv == nil {
		logger.Error("recv map is nil ")
		os.Exit(1)
	}
	for index := 0; index < len(seqs); index++ { // 获取有效的RTT值
		value, ok := recv.Load(seqs[index])
		if !ok {
			logger.Error("recv[seqs[index]] is nil:", zap.Uint64("seqs[index]", seqs[index]), zap.Int("index", index))
			continue
		}
		res, ok := value.(RecvStatic)
		if !ok {
			logger.Error("error value type ,it must be config.RecvStatic", zap.Any("res", res))
			continue
		}
		sta := res
		rtts[index] = math.Floor(sta.RTT*100) / 100
		ttls[index] = sta.TTL
	}
	capRecv := 0
	recv.Range(func(key, value interface{}) bool {
		capRecv++
		return true
	})
	// 最值 方差 四分位数等等
	res := RttToAllStatistics(rtts) //除丢包率，其他均获得
	ttlRes := TTLToStatistics(ttls)
	lossRate := math.Floor((1-float64(len(seqs))/float64(capRecv))*100) / 100 // 丢包率
	//logger.Infof("packet_loss = %v", lossRate)
	res = append(res, lossRate)
	res = append(res, ttlRes...)
	//fmt.Println(len(res), res)
	return res
}

func CellN9eV52Cells(cellN9eV5 []CellN9eV5) []Cell {
	cells := make([]Cell, len(cellN9eV5))
	for i := 0; i < len(cellN9eV5); i++ {
		cells[i].DestIp = cellN9eV5[i].DestIp
		cells[i].Step = cellN9eV5[i].Step
		cells[i].Tags = cellN9eV5[i].Tags.Describe
		cells[i].SourceIp = cellN9eV5[i].SourceName
		cells[i].CounterType = cellN9eV5[i].CounterType
		cells[i].Metric = cellN9eV5[i].Metric
		cells[i].Endpoint = cellN9eV5[i].Endpoint
		cells[i].Timestamp = cellN9eV5[i].Timestamp
		cells[i].Value = cellN9eV5[i].Value
		cells[i].SourceName = cellN9eV5[i].SourceName
	}
	return cells
}
func Cells2CellN9es(cells []Cell) []CellN9eV5 {
	cellN9es := make([]CellN9eV5, 0)
	for i := 0; i < len(cells); i++ {
		if cells[i].Value == -9 {
			if strings.Contains(cells[i].Metric, "loss") {
				cells[i].Value = 1
			} else {
				continue
			}
		}
		if strings.Contains(cells[i].Metric, "rtt") && cells[i].Value > 2000 {
			continue
		}
		Tags := N9e5Tags{
			Ident: cells[i].SourceName,
		}
		tags := strings.Split(cells[i].Tags, ",")
		if len(tags) < 8 {
			Tags.Describe = cells[i].Tags
		} else {
			Tags.Describe = tags[1] + "," + tags[2] + "," + tags[3] + "," + tags[7]
		}
		cellN9e := CellN9eV5{
			Tags:      Tags,
			Timestamp: cells[i].Timestamp,
			Value:     cells[i].Value,
			Metric:    cells[i].Metric,
		}
		cellN9es = append(cellN9es, cellN9e)
	}
	return cellN9es
}
