package config

import (
	"Traceroute/state"
	"fmt"
	"gopkg.in/fsnotify/fsnotify.v1"
	"os"
	"runtime"
	"strings"
)

func DynamicUpdateConfig(confFilename, cfgFilename string, confChan chan Config) {
	//先配置需要监控的文件，再开启监控
	//创建一个监控对象
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		//logger.Error("get file watcher fail:", zap.Error(err))
		logger.Error(fmt.Sprintf("manage\tget file watcher fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return
	}
	var (
		mode0 = true
		mode1 = true
	)
	// windows不支持修改文件权限
	if runtime.GOOS == "linux" {
		chmodTo777(confFilename)
		chmodTo777(cfgFilename)
		mode0, mode1 = checkAuthority(confFilename, cfgFilename)
	}
	// 主监控文件confFilename失效，动态更新程序无法运行
	if mode0 {
		//logger.Info("add to file watcher", zap.String("confFilename", confFilename))
		logger.Info(fmt.Sprintf("manage\tadd config file to watcher\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		//添加要监控的对象，文件或文件夹
		err := watcher.Add(confFilename)
		if err != nil {
			//logger.Error("watcher add fail", zap.String("confFilename", confFilename))
			logger.Error(fmt.Sprintf("manage\tadd config file to watcherfail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			return
		}
	} else {
		//logger.Error("the fileMode of file must over 666, set dynamic update fail", zap.String("", confFilename))
		logger.Error(fmt.Sprintf("manage\tthe fileMode of file must over 666, set dynamic update fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return
	}
	// cfg文件，主要用于读取主机名和IP地址，IP地址也可动态获取
	if mode1 {
		logger.Info(fmt.Sprintf("manage\tadd point file to watcher\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		err := watcher.Add(cfgFilename)
		if err != nil {
			logger.Warn(fmt.Sprintf("manage\tadd point file to watcherfail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		}
	} else {
		logger.Error(fmt.Sprintf("manage\tthe fileMode of file must over 666, set dynamic update fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	}
	//并发防止阻塞主线程，监控配置文件
	go notifyMeasureAgent(watcher, confFilename, cfgFilename, confChan)
}
func notifyMeasureAgent(watcher *fsnotify.Watcher, confFilename, cfgFilename string, confChan chan Config) {
	defer watcher.Close()
	for {
		select {
		case ev := <-watcher.Events:
			switch {
			case ev.Op&fsnotify.Write == fsnotify.Write: //写入文件
				if !strings.EqualFold(ev.Name, cfgFilename) && !strings.EqualFold(ev.Name, confFilename) {
					//logger.Warn("can't support notify this file %v", zap.String("ev.Name", ev.Name))
					logger.Warn(fmt.Sprintf("manage\tcan't support notify this file: %v\tcpu:%v,mem:%v", ev.Name, state.LogCPU, state.LogMEM))
					return
				}
				if len(confChan) == cap(confChan) {
					//logger.Warn("you change config file too fast, it won't be changed ")
					logger.Warn(fmt.Sprintf("manage\tyou change config file too fast, it won't be changed %v\tcpu:%v,mem:%v", ev.Name, state.LogCPU, state.LogMEM))
					continue
				}
				newConf, err := LoadConfig(confFilename, cfgFilename) // 注意读取cfg文件
				if err != nil {
					//logger.Fatal("update config fail", zap.Error(err))
					logger.Fatal(fmt.Sprintf("manage\tupdate config fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
					continue
				}
				confChan <- *newConf

				//logger.Info("it will update config next period, the target conf is the last modify ", zap.String("ev.Name", ev.Name))
			case ev.Op&fsnotify.Create == fsnotify.Create:
				//logger.Info("create")
				logger.Info(fmt.Sprintf("manage\tcreate file\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			}
		case err := <-watcher.Errors:
			if err != nil {
				//logger.Error("notify fail :%v", zap.Error(err))
				logger.Error(fmt.Sprintf("manage\tnotify fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
				return
			}
		}
	}
}

// 权限666 以下时失效
func checkAuthority(confFilename, cfgFilename string) (bool, bool) {
	fileInfo, _ := os.Stat(confFilename)
	mode := fileInfo.Mode()
	//logger.Info("mode", zap.String("confFilename", confFilename), zap.Any("mode", mode))
	fileInfo1, _ := os.Stat(cfgFilename)
	mode1 := fileInfo1.Mode()
	//logger.Info("mode", zap.String("cfgFilename", cfgFilename), zap.Any("mode1", mode1))
	return mode == os.ModePerm, mode1 == os.ModePerm
}

// 修改文件权限
func chmodTo777(filename string) {
	//Open打开一个文件用于读取。如果操作成功，返回的文件对象的方法可用于读取数据；
	//对应的文件描述符具有O_RDONLY模式。如果出错，错误底层类型是*PathError
	f, err := os.Open(filename)
	if err != nil {
		//logger.Error("open fail:", zap.String("filename", filename), zap.Error(err))
		logger.Error(fmt.Sprintf("manage\topen config file fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	}
	defer f.Close()
	fileInfo, err := f.Stat()
	if err != nil {
		panic(err)
	}
	//logger.Debug("file permit %vn", zap.Any("fileInfo.Mode()", fileInfo.Mode()))
	logger.Debug(fmt.Sprintf("manage\tfile mode is :%v\tcpu:%v,mem:%v", fileInfo.Mode(), state.LogCPU, state.LogMEM))

	// 更改权限
	err = f.Chmod(os.ModePerm)
	if err != nil {
		//logger.Error("chmod 777 fail", zap.String("filename", filename), zap.Error(err))
		logger.Error(fmt.Sprintf("manage\tchange file %v mode to 777 fail\tcpu:%v,mem:%v", filename, state.LogCPU, state.LogMEM))
	}
	fileInfo, err = f.Stat()
	if err != nil {
		//logger.Error("get stat fail:", zap.String("filename", filename), zap.Error(err))
		logger.Error(fmt.Sprintf("manage\tget file %vstat fail mode to 777 fail\tcpu:%v,mem:%v", filename, state.LogCPU, state.LogMEM))
	}
}
