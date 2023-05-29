package mylog

import (
	"flag"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"runtime"
	"strings"
)

const (
	//Filename   = "./logs"  // 日志文件路径
	maxSize    = 128  // 每个日志文件保存的最大尺寸 单位：M
	maxBackups = 7    // 日志文件最多保存多少个备份
	maxAge     = 3    // 文件最多保存多少天
	compress   = true // 是否压缩
)

var (
	logLevel string
	logFile  string
	logStd   bool
	win10    = runtime.GOOS == "windows"
)

const (
	defaultLogFileForLinux = "../logs/log"
	defaultLogFile         = "/root/logs/log"
	defaultLogFile2        = "./logs/log2"
	defaultLevel           = "info"
)

func init() {
	if win10 {
		flag.StringVar(&logLevel, "logLevel", defaultLevel, "logLevel is invalid,use INFOLevel replace")
		flag.StringVar(&logFile, "logFile", defaultLogFile, "logFile isn't exist,use stdout replace")
		flag.BoolVar(&logStd, "logStd", true, "copy log output to stdout")
	} else {
		// go run main.go -logLevel info -logStd true
		flag.StringVar(&logLevel, "logLevel", defaultLevel, "logLevel is invalid,useINFO Level replace")
		flag.StringVar(&logFile, "logFile", defaultLogFile, "logFile isn't exist,use stdout replace")
		//flag.StringVar(&logFile, "logFile", defaultLogFileLinux, "logFile isn't exist,use stdout replace")
		flag.BoolVar(&logStd, "logStd", false, "copy log output to stdout")
	}
	//对命令行参数进行解析
	//flag.Parse()
}

func getLevel() zapcore.Level {
	// TODO recover
	tempLevel := zapcore.InfoLevel
	switch {
	case strings.EqualFold(logLevel, "debug"):
		tempLevel = zapcore.DebugLevel
	case strings.EqualFold(logLevel, "info"):
		tempLevel = zapcore.InfoLevel
	case strings.EqualFold(logLevel, "warn"):
		tempLevel = zapcore.WarnLevel
	case strings.EqualFold(logLevel, "error"):
		tempLevel = zapcore.ErrorLevel
	case strings.EqualFold(logLevel, "dpanic"):
		tempLevel = zapcore.DPanicLevel
	case strings.EqualFold(logLevel, "panic"):
		tempLevel = zapcore.PanicLevel
	case strings.EqualFold(logLevel, "fatal"):
		tempLevel = zapcore.FatalLevel
	}
	return tempLevel
}

// zap日志库
//Zap是非常快的、结构化的，分日志级别的Go日志库。
//根据Uber-go Zap的文档，它的性能比类似的结构化日志包更好——也比标准库更快
func ZapConfig() *zap.Logger {
	//NowUnixTime := time.Now().Unix()
	//indexDate := time.Unix(NowUnixTime, 0).Format("20060102")
	fileWriteSyncer := lumberjack.Logger{
		Filename:   logFile,    // 日志文件存放目录，如果文件夹不存在会自动创建
		MaxSize:    maxSize,    // 每个日志文件保存的最大尺寸 单位：M
		MaxBackups: maxBackups, // 日志文件最多保存多少个备份
		MaxAge:     maxAge,     // 文件最多保存多少天
		Compress:   compress,   // 是否压缩
	}
	defer fileWriteSyncer.Close()
	encoderConfig := zapcore.EncoderConfig{
		LevelKey:      "level",
		TimeKey:       "time",
		NameKey:       "logger",
		CallerKey:     "linenum",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalLevelEncoder, // 小写编码器
		//EncodeLevel:    zapcore.CapitalColorLevelEncoder,   // 指定颜色
		EncodeTime:     zapcore.ISO8601TimeEncoder,    // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.StringDurationEncoder, //
		EncodeCaller:   zapcore.FullCallerEncoder,     // 全路径编码器
		EncodeName:     zapcore.FullNameEncoder,
	}
	// 设置日志级别
	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(getLevel())
	var WriteSyncer zapcore.WriteSyncer = nil
	//是否输出到控制台
	if logStd {
		WriteSyncer = zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(&fileWriteSyncer))
	} else {
		WriteSyncer = zapcore.NewMultiWriteSyncer(zapcore.AddSync(&fileWriteSyncer))
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig), // 获取编码器,NewJSONEncoder()输出json格式，NewConsoleEncoder()输出普通文本格式
		WriteSyncer,                              // 打印到控制台和文件
		atomicLevel,                              // 日志级别
	)
	// 开启开发模式，堆栈跟踪
	//AddCaller将日志记录器配置为使用zap调用者的文件名、行号和函数名来注释每条消息。也看到WithCaller。
	caller := zap.AddCaller() // 显示文件名及行号
	development := zap.Development()
	//development输出格式如下
	//2018-10-30T17:14:22.459+0800    DEBUG    development/main.go:7    This is a DEBUG message
	// 设置初始化字段
	filed := zap.Fields()
	// 构造日志
	//默认情况下日志都会打印到应用程序的console界面，但是为了方便查询，
	//可以将日志写入文件，而是使用zap.New()
	return zap.New(core, caller, development, filed)

}
func GetLogger() *zap.Logger {
	return ZapConfig()
}
