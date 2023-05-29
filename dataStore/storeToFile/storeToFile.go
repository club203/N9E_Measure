package storeToFile

import (
	"Traceroute/config"
	"Traceroute/mylog"
	"Traceroute/state"
	"Traceroute/statisticsAnalyse"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	// todo 写绝对路径
	dir      = "./data/"
	icmpFile = dir + "icmp.data"
	logger   = mylog.GetLogger()
)

func openDataFile() *os.File {
	var err error
	_, err = os.Lstat(dir)
	if os.IsNotExist(err) {
		err = os.Mkdir(dir, os.ModeDir)
		if err != nil {
			//logger.Error("mkdir fail: ", zap.String("dir", dir), zap.Error(err))
			logger.Error(fmt.Sprintf("store\tmkdir store data dir fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			return nil
		}
	}
	icmpFileHandle, err := os.OpenFile(icmpFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		//logger.Error("open or create  fail:", zap.String("icmpFile", icmpFile), zap.Error(err))
		logger.Error(fmt.Sprintf("store\topen or create store data file %v fail\tcpu:%v,mem:%v", icmpFile, state.LogCPU, state.LogMEM))
		return nil
	}
	return icmpFileHandle
}
func SavePerPacketToLocal(recv *sync.Map, conf *config.Config, pointIndex int) {

	save(recv, conf, pointIndex)
}
func save(recv *sync.Map, conf *config.Config, pointIndex int) {
	var file *os.File
	defer func() {
		if file != nil {
			file.Close()
		}
		err := recover()
		if err != nil {
			//logger.Error("unknown err ", zap.Any("err", err))
			//logger.Error("unknown err ", zap.Any("err", err))
		}
	}()
	// 获取测量的类型
	protocol := ""
	var buffer bytes.Buffer
	point := conf.Points[pointIndex]
	for index := uint64(1); index <= conf.Points[pointIndex].PeriodPacketNum; index++ {
		value, ok := recv.Load(index)
		if !ok {
			//logger.Error("can't find seq ", zap.Uint64("seqs[index]", index))
			logger.Error(fmt.Sprintf("store\tcan't find seq %v\tcpu:%v,mem:%v", index, state.LogCPU, state.LogMEM))
			continue
		}
		res, ok := value.(statisticsAnalyse.RecvStatic)
		if !ok {
			//logger.Error("error value type ,it must be config.RecvStatic", zap.Any("type ", res))
			logger.Error(fmt.Sprintf("store\terror value type ,it must be config.RecvStatic\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			continue
		}
		res.Alias = fmt.Sprintf("%s_to_%s,percentPacketIntervalMs=%d,periodPacketNum=%d,size=%d,percentPacketIntervalUs=%d,periodNum=%d,periodMs=%d,step=%d,mixture=%v", conf.Data.Hostname, res.Alias, point.PerPacketIntervalMs, point.PeriodPacketNum, point.Size, point.PerPacketIntervalUs, point.PeriodNum, point.PeriodMs, conf.Data.Step, conf.Data.Mixture)
		// 获取测量的类型
		if res.Alias != "" {
			protocol = res.Proto
		}
		//cell, err := yaml.Marshal(res)
		cell, err := json.Marshal(res)

		if err != nil {
			//logger.Error("SavePerPacketToLocal json fail :", zap.Error(err))
			logger.Error(fmt.Sprintf("store\tSavePerPacketToLocal json fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			return
		}
		buffer.WriteString(string(cell) + "\n")
	}
	file = openDataFile()
	//logger.Info("save percent packet data to , protocol: ", zap.String("file.Name", file.Name()), zap.String("protocol", protocol))
	logger.Info(fmt.Sprintf("store\tsave packet data\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	if file == nil {
		//logger.Error("file fd is nil")
		return
	}
	filename := file.Name()
	fileInfo, err := os.Lstat(file.Name())
	if err != nil {
		//logger.Error("unknowable err:", zap.Error(err))
		return
	}
	size := fileInfo.Size()
	// 数据流达到100MB就另存
	if size > conf.Data.DataSizeMB*1024*1024 { //dataSizeMBM就另存
		saveTime := time.Now()
		file.Close() // 关闭旧文件 不关闭，无法修改文件名
		newFilename := fmt.Sprintf("%s%s-%04d-%02d-%02d-%02d-%02d",
			dir,
			protocol,
			saveTime.Year(),
			saveTime.Month(),
			saveTime.Day(),
			saveTime.Hour(),
			saveTime.Minute(),
		)
		err = os.Rename(filename, newFilename)
		if err != nil {
			//logger.Error("rename fail:", zap.Error(err))
			return
		}

		file, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModeAppend)
		if err != nil {
			//logger.Error("open or create fail:", zap.String("filename", filename), zap.Error(err))
		}
		err = file.Chmod(os.ModePerm)
		if err != nil {
			//logger.Error("chmod file fail:", zap.String("filename", filename), zap.Error(err))
		}
	}
	n, err := buffer.WriteTo(file)
	if n <= 0 {
		//logger.Error("write to %v fail:%v", zap.String("filename", filename), zap.Error(err))
		return
	}

	err = file.Close()
	if err != nil {
		//logger.Warn(filename + " close fail")
	}
	deleteExpiredData(conf.Data.Hostname)
}

//deleteExpiredData 根据编写的文件名时间来删除过期数据
func deleteExpiredData(hostname string) {
	fileInfos, _ := ioutil.ReadDir(dir)

	for _, info := range fileInfos {
		if strings.EqualFold(info.Name(), "icmp.data") || strings.EqualFold(info.Name(), "udp.data") || strings.EqualFold(info.Name(), "tcp.data") {
			continue
		}
		var parse = time.Now()
		var err error = nil
		switch {
		case strings.Contains(info.Name(), "icmp-"):
			parse, err = time.Parse("icmp-2006-01-02-15-04", info.Name())
		case strings.Contains(info.Name(), "udp-"):
			parse, err = time.Parse("udp-2006-01-02-15-04", info.Name())
		case strings.Contains(info.Name(), "tcp-"):
			parse, err = time.Parse("tcp-2006-01-02-15-04", info.Name())
		}
		if err != nil {
			continue
		}
		sub := time.Now().Sub(parse)
		isDel := false
		if strings.Contains(hostname, "SDZX") {
			isDel = sub > time.Duration(24*10)*time.Hour
		} else {
			isDel = sub > time.Duration(24*10)*time.Hour
		}

		if isDel {
			_ = os.Remove(dir + info.Name())
		}
	}
}
