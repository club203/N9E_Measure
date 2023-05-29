package main

import (
	"Traceroute/config"
	"Traceroute/measure"
	"Traceroute/mylog"
	"Traceroute/state"
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"
)

const (
	winDirOfConfig       = "./config/MeasureAgent_trace.yml"
	winDirOfCFG          = "/root/cfg.yaml"
	linuxDirOfConfig     = "/root/MeasureAgent_trace.yml"
	linuxDirOfConfigTest = "/root/MeasureAgent_trace.yml"
	linuxDirOfCFG        = "/root/point.yml"
)

var (
	configurationFilename string
	win10                 = runtime.GOOS == "windows"
	logger                = mylog.GetLogger()
	BJValues              = make([]string, 15)
)

func init() {
	if win10 {
		flag.StringVar(&configurationFilename, "config", winDirOfConfig, "config is invalid, use default.json replace")
	} else {
		flag.StringVar(&configurationFilename, "config", linuxDirOfConfig, "config is invalid, use default.json replace")
	}
	//flag.Parse()
}

func main() {
	fmt.Println("version 2.0")
	measure.Prepare = time.Now().UnixNano()
	go state.CheckCPUAndMem()
	measure.Test = false
	if measure.Test {
		configurationFilename = linuxDirOfConfigTest
	} else {
		//go func() {
		//	time.Sleep(590 * time.Second) //空出来10s
		//	dao.Detail += "Timeout;"
		//	storeToFile.AppendFile("./CrontabStatus.txt", "")
		//	logger.Error("After running for more than 600 s, the program exits!!!!!!!!!!!!!!!!!,RetryTime: " + strconv.Itoa(dao.RetryTime))
		//	os.Exit(1)
		//}()
	}
	configurationFilename = linuxDirOfConfigTest

	//go func() {
	//	//logger.Info("%v", zap.Error(http.ListenAndServe("0.0.0.0:7070", nil)))
	//	logger.Info("%v", zap.Error(http.ListenAndServe("0.0.0.0:7070", nil)))
	//}()
	//flag.Parse()
	var cfgFilename string
	if win10 {
		cfgFilename = "./config/cfg.yaml"
	} else {
		cfgFilename = "/root/point.yml"
	}
	conf, err := config.LoadConfig(configurationFilename, cfgFilename)
	if err != nil {
		log.Fatalf("manage\tfail to load config\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM)
		return
	}
	//TODO 开启ICMP echo Request接收，目的是拿到Ping测量的TTL
	//measure
	measure.StartTrace(conf)
	//logger.Info("end measuring ...", zap.String("time", time.Since(start).String()))
	logger.Info(fmt.Sprintf("manage\tend measuring...\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	//if dao.TransferRestFile(conf.Data.Hostname) == 0 {
	//	dao.Detail += "NoRest;"
	//}
	//if !measure.Test {
	//	storeToFile.AppendFile("./CrontabStatus.txt", "")
	//}
}
