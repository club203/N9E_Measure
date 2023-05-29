package listening

import (
	"Traceroute/config"
	"Traceroute/mylog"
	"Traceroute/state"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var logger = mylog.GetLogger()
var once sync.Once

//执行shell命令
func ShellCmd(command string) (error, string) {
	cmd := exec.Command(shellToUse, "-c", command)
	output, err := cmd.CombinedOutput()
	return err, string(output)
}

//由于socat回显会产生一些无法释放的TCP链接（状态为CLOSE_WAIT）
//所以需要定时清除这些socat
//但根据netstat无法获取全部有问题的socat进程，所以只能用ps或者pgrep，留20个
func clearCloseWaitTcp() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err, s := ShellCmd("ps aux --sort=start_time|grep socat|awk '{print $2}' | head -`expr \\`ps aux --sort=start_time|grep socat|awk '{print $2}'|wc|awk '{print $1}'\\` - 20` | xargs kill -s 9")
			if err != nil && !strings.Contains(s, "") {
				//logger.Error("clear CLOSE_WAIT TCP fail:%v,%v", zap.String("output", s), zap.Error(err))
				logger.Error(fmt.Sprintf("listen\tclear CLOSE_WAIT TCP fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
				if strings.Contains(s, "head:") {
					//logger.Info("the count of CLOSE_WAIT TCP less than 20")
					logger.Info(fmt.Sprintf("listen\tthe count of CLOSE_WAIT TCP less than 20\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
				}
			} else {
				//logger.Info("clear CLOSE_WAIT TCP")
				logger.Info(fmt.Sprintf("listen\tclear CLOSE_WAIT TCP\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			}
		}
	}
}
func socatCmdCheck(udpPort, tcpPort string) (bool, bool) {
	tcpNet, udpNet := false, false
	err, _ := ShellCmd("yum -y install socat")
	if err != nil {
		//logger.Error("yum -y install socat fail :", zap.Error(err), zap.String("out", out))
	} else {
		//logger.Info("start socat success")
		logger.Info(fmt.Sprintf("listen\tstart socat success\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	}
	err, _ = ShellCmd("apt-get -y install socat")
	if err != nil {
		//logger.Info("install socat fail", zap.String("output", out), zap.Error(err))
	} else {
		logger.Info(fmt.Sprintf("listen\tstart socat success\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	}
	//netstat -nltp 用于查看Linux服务器上当前机器监听的端口信息
	err, _ = ShellCmd("netstat -ntlp|grep " + tcpPort)
	if err == nil {
		tcpNet = true
	}
	err, _ = ShellCmd("netstat -nulp|grep " + udpPort)
	if err == nil {
		udpNet = true
	}
	return udpNet, tcpNet
}
func ServerListen(conf *config.Config) {
	udpPort := strconv.Itoa(conf.Data.RUDPP)
	tcpPort := strconv.Itoa(conf.Data.RTCPP)
	//logger.Info("listening ...", zap.Bool("Continuity mode", conf.Data.IsContinuity), zap.String("tcpPort", tcpPort), zap.String("udpPort", udpPort))
	logger.Info(fmt.Sprintf("listen\tstart listening ...\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	if runtime.GOOS == "windows" {
		return
	}
	// TODO 需要测试
	once.Do(func() {
		go clearCloseWaitTcp()
	})
	if conf.Data.IsContinuity {
		//没有IP地址
		//zm暂时注释，下面两行不要删
		//go tcpListen(":" + tcpPort)
		//go udpListen(":" + udpPort)
	} else if !conf.Data.IsContinuity {
		udpNet, tcpNet := socatCmdCheck(udpPort, tcpPort) //查看socat是否安装以及监听端口
		// 避免执行多个socat
		if !tcpNet {
			go socatTCP(tcpPort)
		}
		if !udpNet {
			go socatUDP(udpPort)
		}
	} else {
		//logger.Warn("can't support this platform,you may get wrong result : ", zap.String("platform", runtime.GOOS))
		logger.Warn(fmt.Sprintf("listen\tcan't support this platform,you may get wrong result :%v\tcpu:%v,mem:%v", runtime.GOOS, state.LogCPU, state.LogMEM))
	}
}
