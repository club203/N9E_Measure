package state

import (
	"Traceroute/mylog"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var logger = mylog.GetLogger()
var LogCPU float64
var LogMEM int64
var ProcessName = "/root/ICMPzm/ICMP"

func AliasByPointYml() (result string) {
	str2 := GetCmdOut("cat /root/point.yml")
	for i := 0; i < len(str2); i++ {
		if strings.Contains((str2)[i], "alias") {
			return strings.Split((str2)[i], " ")[1]
		}
	}
	return ""
}
func AddressByPointYml() (result string) {
	str2 := GetCmdOut("cat /root/point.yml")
	for i := 0; i < len(str2); i++ {
		if strings.Contains((str2)[i], "address") {
			return strings.Split((str2)[i], " ")[1]
		}
	}
	return ""
}
func GetCmdOut(cmdString string) (CmdOutPut []string) {
	cmd := exec.Command("/bin/bash", "-c", cmdString)
	//创建获取命令输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		//fmt.Printf("Error:can not obtain stdout pipe for command:%s\n", err)
		return
	}
	//执行命令
	if err := cmd.Start(); err != nil {
		//fmt.Println("Error:The command is err,", err)
		return
	}
	//读取所有输出
	contain, err := ioutil.ReadAll(stdout)
	if err != nil {
		//fmt.Println("ReadAll Stdout:", err.Error())
		return
	}
	if err := cmd.Wait(); err != nil {
		//fmt.Println("wait:", err.Error())
		return
	}

	str := string(contain)
	str2 := strings.Split(str, "\n")
	return str2
}

func PushData(Data []*MetricValue) (err error) {
	url := "http://106.3.133.6/api/transfer/push"
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
	//logger.Info("this time of push finished:", zap.String(pushResponse.Dat, pushResponse.Err))
	return
}

func CheckCPUAndMem() {
	Alias := AliasByPointYml()
	CpuMetric := "proc.ICMPSimply.cpu." + Alias
	MemMetric := "proc.ICMPSimply.rss." + Alias
	Endpoint := Alias
	const Step = 20
	const Pack = 5 //得到了Pack次数据了再一起push她
	begin := 0     //判断现在存了几次数据

	var DataMetric []*MetricValue
	items := make([]MetricValue, Pack*2, Pack*2) //不能直接对指针操作，西药先append进去
	for i := 0; i < Pack*2; i++ {
		DataMetric = append(DataMetric, &items[i])
	}
	for i := 0; i < Pack; i++ {
		DataMetric[i].Metric = CpuMetric
		DataMetric[i+Pack].Metric = MemMetric
	}
	for i := 0; i < Pack*2; i++ {
		DataMetric[i].Step = int64(Step)
		DataMetric[i].Endpoint = Endpoint
	}
	for {
		CmdOutPut := GetCmdOut("ps aux|grep " + ProcessName)
		CheckTime := time.Now().Unix()
		DataMetric[begin].Timestamp = CheckTime
		DataMetric[begin+Pack].Timestamp = CheckTime
		for _, i2 := range CmdOutPut {
			i3 := strings.Fields(i2)
			if len(i3) == 11 && i3[10] == ProcessName {
				CPU := i3[2]
				RSS := i3[5]

				DataMetric[begin].ValueUntyped, _ = strconv.ParseFloat(CPU, 64)
				DataMetric[begin+Pack].ValueUntyped, _ = strconv.ParseFloat(RSS, 64)
				LogCPU, _ = strconv.ParseFloat(CPU, 64)
				LogMEM, _ = strconv.ParseInt(RSS, 10, 64)
				//fmt.Println(begin," ",DataMetric[begin].ValueUntyped)
				if begin == Pack-1 {
					begin = 0
					err := PushData(DataMetric)
					//fmt.Println(err)
					if err != nil {
						//logger.Error("push fail:", zap.Error(err))
					}
					//logger.Info("collect Pack times, push to n9e")
				} else {
					begin++
				}
				break
			}
		}
		time.Sleep(time.Second * Step) //Step秒一睡
		//Step不为const时就要用for：
		//invalid operation: time.Second * Step (mismatched types time.Duration and int)
	}
}
