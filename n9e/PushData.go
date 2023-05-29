package n9e

import (
	"Traceroute/mylog"
	"Traceroute/statisticsAnalyse"
	"bytes"
	"encoding/json"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

var logger = mylog.GetLogger()

func PushData(Data []*MetricValue) (err error) {
	url := "http://106.3.133.5/api/transfer/push"
	jsonItems, err := json.Marshal(Data)
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonItems))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)
	body, _ := ioutil.ReadAll(resp.Body)
	var pushResponse PushResponse
	err = json.Unmarshal(body, &pushResponse)
	if err != nil {
		return
	}
	logger.Info("this time of push finished:", zap.String(pushResponse.Dat, pushResponse.Err))
	return
}

func PushDataV3(Data []statisticsAnalyse.Cell) (err error) {
	metric := make([]*MetricValue, len(Data))
	for i := range Data {
		cell := Data[i]
		metric[i] = &MetricValue{
			Endpoint:     cell.Endpoint,
			Metric:       cell.Metric,
			Timestamp:    cell.Timestamp,
			Step:         cell.Step,
			ValueUntyped: cell.Value,
			Tags:         cell.Tags,
			CounterType:  cell.CounterType,
		}
	}
	return PushData(metric)
}

func PushDataV5ForSuccess(Data []statisticsAnalyse.CellN9eV5) (err error, NewRequestTime, Do int64, retry int) {
	TryTime := 3
	for i := 0; i < TryTime; i++ {
		err, NewRequestTime, Do = PushDataV5(Data)
		if err == nil {
			return err, NewRequestTime, Do, i
		}
	}
	return err, NewRequestTime, Do, TryTime
}
func PushDataV5(Data []statisticsAnalyse.CellN9eV5) (err error, NewRequestTime, Do int64) {
	url := "http://106.3.133.5:19000/opentsdb/put"
	jsonItems, err := json.Marshal(Data)
	if err != nil {
		return
	}
	time1 := time.Now().UnixNano()
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonItems))
	if req == nil {
		return
	}
	NewRequestTime = time.Now().UnixNano() - time1
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	time1 = time.Now().UnixNano()
	resp, err := client.Do(req)
	Do = time.Now().UnixNano() - time1
	if err != nil {
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var pushResponseV5 PushResponseV5
	err = json.Unmarshal(body, &pushResponseV5)
	if err != nil {
		return
	}
	logger.Info("this time of push finished:", zap.String("Fail nums", string(rune(pushResponseV5.Fail))))
	return
}
