package monitor

import (
	"Traceroute/mylog"
	"Traceroute/network"
	"fmt"
	"golang.org/x/net/context"
)

type SystemMonitor struct {
	SNMPProxy  network.SNMPClient
	PreTraffic []int
	CurTraffic []int
}

var (
	logger = mylog.GetLogger()
)

func (sm *SystemMonitor) InitMonitor(ip string, retry, timeout, req int) {
	var err error
	sm.SNMPProxy, err = network.NewSNMPClientByConnectionData(context.Background(), ip, &network.SNMPConnectionData{
		Communities:              []string{"snmp-test"},
		Versions:                 []string{"2c"},
		Ports:                    []int{161},
		DiscoverRetries:          &retry,
		DiscoverTimeout:          &timeout,
		DiscoverParallelRequests: &req,
	})
	if err != nil {
		//logger.Error()
		fmt.Println(fmt.Sprintf("SNMP Client init error, %s", err.Error()))
	}
}

func (sm *SystemMonitor) GetBandwidth(ip string, oid network.OID) []int {
	sm.InitMonitor(ip, 3, 2000, 1)
	if sm.SNMPProxy == nil {
		//logger.Error()
		fmt.Println(fmt.Sprintf("SNMP Client has not be init"))
		return nil
	}
	// 1.3.6.1.2.1.2.2.1.10 入口流量OID标识符 ——https://oidref.com/
	// 1.3.6.1.2.1.2.2.1.16 出口流量OID标识符 ——https://oidref.com/
	responses, err := sm.SNMPProxy.SNMPWalk(context.Background(), oid)
	if err != nil {
		//logger.Error(fmt.Sprintf(""))
		fmt.Println(fmt.Sprintf("no response, %s", err.Error()))
		return nil
	}
	var curTurn []int
	for index := range responses {
		value, _ := responses[index].GetValue()
		v, _ := value.Int()
		curTurn = append(curTurn, v)
	}
	return curTurn
}

func (sm *SystemMonitor) CalculateBandwidth(ip string) map[string]float64 {
	if sm.PreTraffic == nil {
		logger.Error("lake data")
		return nil
	}
	desc := sm.GetNetworkDesc(ip)
	result := make(map[string]float64, len(sm.PreTraffic))
	for index := range sm.CurTraffic {
		traffic := float64(sm.CurTraffic[index] - sm.PreTraffic[index])
		deltaTime := float64(60)
		result[desc[index]] = (traffic * 8) / (deltaTime * 1024 * 1024)
	}
	return result
}

func (sm *SystemMonitor) GetNetworkDesc(ip string) []string {
	sm.InitMonitor(ip, 3, 2000, 1)
	if sm.SNMPProxy == nil {
		//logger.Error()
		fmt.Println(fmt.Sprintf("SNMP Client has not be init"))
		return nil
	}
	// 1.3.6.1.2.1.2.2.1.2 网络接口描述符
	responses, err := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.2.1.2.2.1.2")
	if err != nil {
		//logger.Error(fmt.Sprintf(""))
		fmt.Println(fmt.Sprintf("no response, %s", err.Error()))
		return nil
	}
	var result []string
	for index := range responses {
		value, _ := responses[index].GetValue()
		result = append(result, value.String())
	}
	return result
}

// GetCPU hrProcessorLoad 1.3.6.1.2.1.25.3.3.1.2
func (sm *SystemMonitor) GetCPU(ip string) *float64 {
	sm.InitMonitor(ip, 3, 2000, 1)
	if sm.SNMPProxy == nil {
		//logger.Error()
		fmt.Println(fmt.Sprintf("SNMP Client has not be init"))
		return nil
	}
	// hrProcessorLoad 1.3.6.1.2.1.25.3.3.1.2
	responses, err := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.2.1.25.3.3.1.2")
	if err != nil {
		//logger.Error(fmt.Sprintf(""))
		fmt.Println(fmt.Sprintf("no response, %s", err.Error()))
		return nil
	}
	r := 0.0
	for index := range responses {
		value, _ := responses[index].GetValue()
		f, _ := value.Float64()
		r = +f
	}
	r = r / 4
	return &r
}

// GetStore
// hrStorageUsed(每个分区使用过的簇的个数) 1.3.6.1.2.1.25.2.3.1.6
// hrStorageSize(每个分区总的簇的个数) 1.3.6.1.2.1.25.2.3.1.5
// hrStorageAllocationUnits(每个分区对应的簇的大小) 1.3.6.1.2.1.25.2.3.1.4
// 使用过的硬盘大小 = 使用过的簇的个数 * 每个簇的大小
// 硬盘总大小 = 硬盘总的簇的个数 * 每个簇的大小
// 硬盘利用率 = （使用过的硬盘大小 / 硬盘总大小） * 100%
func (sm *SystemMonitor) GetStore(ip string) *float64 {
	sm.InitMonitor(ip, 3, 2000, 1)
	if sm.SNMPProxy == nil {
		//logger.Error()
		fmt.Println(fmt.Sprintf("SNMP Client has not be init"))
		return nil
	}
	used, err1 := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.2.1.25.2.3.1.6")
	size, err2 := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.2.1.25.2.3.1.5")
	allocUnit, err3 := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.2.1.25.2.3.1.4")
	if err1 != nil || err2 != nil || err3 != nil {
		fmt.Println(fmt.Sprintf("no response"))
	}
	partition := len(used)
	totalUsed := 0.0
	totalAlloc := 0.0
	for i := 0; i < partition; i++ {
		u, _ := used[i].GetValue()
		s, _ := size[i].GetValue()
		a, _ := allocUnit[i].GetValue()
		uValue, _ := u.Float64()
		sValue, _ := s.Float64()
		aValue, _ := a.Float64()
		totalUsed += uValue * aValue
		totalAlloc += sValue * aValue
	}
	result := totalUsed / totalAlloc
	return &result
}

// GetMemory
// memTotalReal 1.3.6.1.4.1.2021.4.5
// memAvailReal 1.3.6.1.4.1.2021.4.6
// memShared：1.3.6.1.4.1.2021.4.13
// memBuffer：1.3.6.1.4.1.2021.4.14
// memCached：1.3.6.1.4.1.2021.4.15
func (sm *SystemMonitor) GetMemory(ip string) *float64 {
	sm.InitMonitor(ip, 3, 2000, 1)
	if sm.SNMPProxy == nil {
		//logger.Error()
		fmt.Println(fmt.Sprintf("SNMP Client has not be init"))
		return nil
	}
	total, err := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.4.1.2021.4.5")
	avail, err := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.4.1.2021.4.6")
	share, err := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.4.1.2021.4.13")
	buf, err := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.4.1.2021.4.14")
	cache, err := sm.SNMPProxy.SNMPWalk(context.Background(), "1.3.6.1.4.1.2021.4.15")
	if err != nil {
		fmt.Println(fmt.Sprintf("no response, %s", err.Error()))
		return nil
	}
	t, _ := total[0].GetValue()
	a, _ := avail[0].GetValue()
	s, _ := share[0].GetValue()
	b, _ := buf[0].GetValue()
	c, _ := cache[0].GetValue()
	tValue, _ := t.Float64()
	aValue, _ := a.Float64()
	sValue, _ := s.Float64()
	bValue, _ := b.Float64()
	cValue, _ := c.Float64()
	result := (aValue - bValue - cValue) / tValue
	fmt.Println(sValue)
	return &result
}
