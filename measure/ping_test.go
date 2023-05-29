package measure

import (
	"Traceroute/config"
	"Traceroute/state"
	"log"
	"testing"
)

func Test_Ping(t *testing.T) {
	configurationFilename := "/home/Traceroute/config/MeasureAgent_trace.yml"
	cfgFilename := "/home/Traceroute/config/cfg.yaml"
	conf, err := config.LoadConfig(configurationFilename, cfgFilename)
	if err != nil {
		log.Fatalf("manage\tfail to load config\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM)
		return
	}
	confChan := make(chan config.Config, 10)
	StartPing(conf, confChan)
}

func Test_Trace(t *testing.T) {
	configurationFilename := "/home/Traceroute/config/MeasureAgent_trace.yml"
	cfgFilename := "/home/Traceroute/config/cfg.yaml"
	conf, err := config.LoadConfig(configurationFilename, cfgFilename)
	if err != nil {
		log.Fatalf("manage\tfail to load config\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM)
		return
	}
	StartTrace(conf)
}
