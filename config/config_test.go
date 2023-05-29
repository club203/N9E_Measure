package config

import (
	"fmt"
	"testing"
)

func Test_Traffic(t *testing.T) {
	conf, err := LoadConfig("/Users/atractylodis/Documents/go-project/Traceroute/config/MeasureAgent_trace.yml",
		"/Users/atractylodis/Documents/go-project/Traceroute/config/cfg.yaml")
	if err != nil {
		fmt.Println(err.Error())
	}
	calculate := BandwidthPreCalculate(conf, false, 1.0, 1)
	fmt.Println(calculate)
}
