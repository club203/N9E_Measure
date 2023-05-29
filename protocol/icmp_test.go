package protocol

import (
	"sync"
	"testing"
)

func init() {

}

func BenchmarkContextSwitch(b *testing.B) {
	var wg sync.WaitGroup
	begin := make(chan struct{})
	channel := make(chan struct{})

	numGoRoutime := 6
	wg.Add(numGoRoutime)
	numGoRoutime -= 1
	sender := func() {
		defer wg.Done()
		<-begin
		for i := 0; i < numGoRoutime*b.N; i++ {
			channel <- struct{}{}
		}
	}
	receiver := func() {
		defer wg.Done()
		<-begin
		for i := 0; i < b.N; i++ {
			<-channel
		}
	}
	go sender()

	for j := 0; j < numGoRoutime; j++ {
		go receiver()
	}
	b.StartTimer()
	close(begin)
	wg.Wait()
}

//func BenchmarkSendICMPPacket(b *testing.B) {
//	logger := mylog.GetLogger()
//	PercentPacketIntervalMs := 5
//	duration := time.Duration(PercentPacketIntervalMs) * time.Millisecond
//	b.ResetTimer()
//	var cost int64 = 0
//	testFun := func() {
//		for i := 0; i < b.N; i++ {
//			start := time.Now().UnixNano()
//			time.Sleep(duration)
//			end := time.Now().UnixNano()
//			cost = cost + end - start - int64(duration)
//		}
//	}
//	testFun()
//	cost /= int64(b.N)
//	logger.Info("耗时", zap.Int64("us", cost/1e3))
//	b.StopTimer()
//}

//func BenchmarkSendICMPPacket(b *testing.B) {
//	var finish = make(chan bool, 1)
//	var recv = new(sync.Map)
//	var intervalMs int64 = 0
//	var intervalUs int64 = 0
//	mes := &Message{
//		Address:                 "139.9.0.76",
//		Size:                    1024,
//		Sequence:                1,
//		Protocol:                "Icmp",
//		IsFinish:                finish,
//		Alias:                   "TX_Shanghai2",
//		PeriodPacketNum:         100,
//		PercentPacketIntervalMs: intervalMs,
//		PercentPacketIntervalUs: intervalUs,
//		Step:                    0,
//		Hostname:                "TX_BJ1",
//		PeriodMs:                0,
//	}
//	conn, _ := net.DialTimeout("ip:Icmp", mes.Address, time.Second*2)
//	//b.Logf("理论时间:%vms", int64(mes.IPNumPerMs)*mes.msPerCNet)
//	defer conn.Close()
//	//b.N = int(mes.IPNumPerMs)
//	duration := time.Duration(mes.PercentPacketIntervalMs)*time.Millisecond + time.Duration(mes.PercentPacketIntervalUs)*time.Nanosecond*1000
//
//	b.ResetTimer()
//	for i := 0; i < b.N; i++ {
//		//b.Run("Icmp send", func(b *testing.B) {
//		SendICMPPacket(mes, conn, recv)
//		time.Sleep(duration)
//		//})
//	}
//	b.StopTimer()
//
//}
