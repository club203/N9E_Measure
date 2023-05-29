package monitor

import (
	"Traceroute/n9e"
	"fmt"
	"testing"
	"time"
)

func Test_SNMP_GetBandwidth(t *testing.T) {
	outMonitor := &SystemMonitor{}
	inMonitor := &SystemMonitor{}
	for {
		outMonitor.CurTraffic = outMonitor.GetBandwidth("106.3.133.2", "1.3.6.1.2.1.2.2.1.16")
		inMonitor.CurTraffic = inMonitor.GetBandwidth("106.3.133.2", "1.3.6.1.2.1.2.2.1.10")
		out := outMonitor.CalculateBandwidth("106.3.133.2")
		in := inMonitor.CalculateBandwidth("106.3.133.2")
		fmt.Println(fmt.Sprintf("out bandwidth: %v", out))
		fmt.Println(fmt.Sprintf("in bandwidth: %v", in))
		time.Sleep(time.Minute)
		outMonitor.PreTraffic = outMonitor.CurTraffic
		inMonitor.PreTraffic = inMonitor.CurTraffic
	}
}

func Test_SNMP_GetCPU(t *testing.T) {
	monitor := &SystemMonitor{}
	for {
		cpu := monitor.GetCPU("106.3.133.2")
		fmt.Println(*cpu)
		err := n9e.Collect("SDZX_BJ", *cpu, time.Now().Unix(), int64(60), "system.SDZX_BJ.cpu")
		if err != nil {
			return
		}
		time.Sleep(time.Minute)
	}
}

func Test_SNMP_GetDisk(t *testing.T) {
	monitor := &SystemMonitor{}
	for {
		disk := monitor.GetStore("106.3.133.2")
		fmt.Println(fmt.Sprintf("%f%%", (*disk)*100))
		time.Sleep(time.Minute)
	}
}
