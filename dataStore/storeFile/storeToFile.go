package storeFile

import (
	"Traceroute/config"
	"Traceroute/mylog"
	"bufio"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	roundTsMap       = make(map[int]string)
	dataRootDir      = "${label}_${create_ts}/"
	dirTemplate      = "${ts}_${round}/${type}/${net_A}/"
	fileNameTemplate = "${net_A}.txt"
	pingFileLock     sync.Mutex
	traceFileLock    sync.Mutex
	logger           = mylog.GetLogger()
	createTs         = time.Unix(time.Now().Unix(), 0).Format("20060102_1504")

	once sync.Once
)

func getFile(netId, dataType string, roundCnt int) *os.File {
	if _, ok := roundTsMap[roundCnt]; !ok {
		// 不存在
		roundTsMap[roundCnt] = time.Unix(time.Now().Unix(), 0).Format("20060102_1504")
	}
	nowRoundTs := roundTsMap[roundCnt]
	cfg := config.GetConfig()

	tmpDir := dataRootDir
	if strings.Contains(dataRootDir, "$") {
		// 拼接
		dataRootDir = path.Join(cfg.Setting.OutPutDir, tmpDir)
		dataRootDir = strings.Replace(dataRootDir, "${label}", cfg.Setting.Label, -1)
		dataRootDir = strings.Replace(dataRootDir, "${create_ts}", createTs, -1)
		tmpDir = dataRootDir
	}

	tmpDir = path.Join(tmpDir, dirTemplate)
	tmpDir = strings.Replace(tmpDir, "${ts}", nowRoundTs, -1)
	tmpDir = strings.Replace(tmpDir, "${round}", strconv.Itoa(roundCnt), -1)
	tmpDir = strings.Replace(tmpDir, "${type}", dataType, -1)
	tmpDir = strings.Replace(tmpDir, "${net_A}", strings.Split(netId, ".")[0], -1)

	if _, err := os.Stat(tmpDir); err != nil {
		err := os.MkdirAll(tmpDir, os.ModePerm)
		if err != nil {
			logger.Error("Error creating directory")
		}
	}

	fileName := path.Join(tmpDir, fileNameTemplate)
	fileName = strings.Replace(fileName, "${net_A}", strings.Split(netId, ".")[0], -1)

	// 检查是否已经存在
	file, _ := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModeAppend)
	return file
}

func SavePingData(netId string, data string, roundCnt int) {
	pingFile := getFile(netId, "ping", roundCnt)
	write := bufio.NewWriter(pingFile)
	_, _ = write.WriteString(data)
	_ = write.Flush()
	_ = pingFile.Close()
}

func SaveTraceData(netId string, data string, roundCnt int) {
	traceFileLock.Lock()
	defer traceFileLock.Unlock()

	traceFile := getFile(netId, "trace", roundCnt)

	write := bufio.NewWriter(traceFile)
	_, _ = write.WriteString(data)
	_ = write.Flush()
	_ = traceFile.Close()
}

//zipExpiredData 根据编写的文件名时间来压缩数据
func zipExpiredData() {
	ticker := time.NewTicker(time.Duration(4) * time.Hour)
	defer ticker.Stop()
	for {
		<-ticker.C
		if strings.Contains(dataRootDir, "$") {
			return
		}
		fileInfos, _ := ioutil.ReadDir(dataRootDir)
		// 获取最大轮
		maxCnt := 0
		for _, info := range fileInfos {
			dirName := info.Name()
			if strings.Contains(dirName, "tar") {
				continue
			}
			splits := strings.Split(dirName, "_")
			if len(splits) != 3 {
				logger.Error("文件夹名字错误", zap.String("文件名", dirName), zap.String("路径", dataRootDir))
				continue
			}
			cnt, _ := strconv.Atoi(splits[2])
			if cnt > maxCnt {
				maxCnt = cnt
			}
		}
		// 保留最后3轮的不压缩
		remainCnt := 1
		for _, info := range fileInfos {
			dirName := info.Name()
			if strings.Contains(dirName, "tar") {
				continue
			}
			splits := strings.Split(dirName, "_")
			if len(splits) != 3 {
				logger.Error("文件夹名字错误", zap.String("文件名", dirName), zap.String("路径", dataRootDir))
				continue
			}
			cnt, _ := strconv.Atoi(splits[2])
			if cnt >= maxCnt-remainCnt {
				continue
			}
			dirName = path.Join(dataRootDir, dirName)
			command := "tar -zcf ${file}.tar.gz ${file} --remove-files"
			command = strings.Replace(command, "${file}", dirName, -1)
			logger.Info("压缩文件夹", zap.String("dir name", dirName))
			cmd := exec.Command("/bin/bash", "-c", command)
			err := cmd.Run()
			if err != nil {
				logger.Error("压缩文件夹错误", zap.Error(err), zap.String("命令", command))
			}
		}
	}
}

func init() {
	go zipExpiredData()
}
