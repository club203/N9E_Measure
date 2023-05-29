package statisticsAnalyse

import (
	"sort"
)

// rtt及jitter均值 方差 最小值 最大值 四分位数一 四分位数二 四分数三
func RttToAllStatistics(rtts []float64) []float64 {
	sort.Float64s(rtts)
	preRtts := preRtts(&rtts)

	rttAvg := average(&preRtts)
	rttVariance := variance(&preRtts, rttAvg)
	rttQuantile25, rttQuantile50, rttQuantile75 := quantile(&preRtts)
	rttMin := preRtts[0]
	rttMax := preRtts[len(preRtts)-1]

	jitters := jitter(&preRtts, rttAvg)
	jitterAvg := average(&jitters)
	jitterVariance := variance(&jitters, jitterAvg)
	jitterQuantile25, jitterQuantile50, jitterQuantile75 := quantile(&jitters)
	jitterMin := jitters[0]
	jitterMax := jitters[len(preRtts)-1]

	//fmt.Println("rtt: ", rtts)
	//fmt.Println(rtt_avg, rtt_variance, rtt_min, rtt_max)
	//fmt.Println("rtt_quantile", rtt_quantile25, rtt_quantile50, rtt_quantile75)
	//fmt.Println("jitter", jitters)
	//fmt.Println(jitter_avg, jitter_variance, jitter_min, jitter_max)
	//fmt.Println("jitter_quantile", jitter_quantile25, jitter_quantile50, jitter_quantile75)
	//fmt.Println("loss rate", loss_rate)

	//					1			2			3		4			5				6				7
	return []float64{rttAvg, rttVariance, rttMin, rttMax, rttQuantile25, rttQuantile50, rttQuantile75,
		//		8				9			10			11				12					13				14
		jitterAvg, jitterVariance, jitterMin, jitterMax, jitterQuantile25, jitterQuantile50, jitterQuantile75,
	}

}

func TTLToStatistics(ttls []int) []float64 {
	sort.Ints(ttls)
	ttLs := preTTLs(&ttls)
	ttlAvg := average(&ttLs)
	ttlVar := variance(&ttLs, ttlAvg)
	ttlQuantile25, ttlQuantile50, ttlQuantile75 := quantile(&ttLs)
	ttlMin := ttLs[0]
	ttlMax := ttLs[len(ttLs)-1]
	return []float64{ttlAvg, ttlVar, ttlMin, ttlMax, ttlQuantile25, ttlQuantile50, ttlQuantile75}
}

func preRtts(rtts1 *[]float64) []float64 {
	var rtts2 []float64
	for _, rtt := range *rtts1 {
		if rtt > 0 {
			rtts2 = append(rtts2, rtt)
		}
	}
	if len(rtts2) == 0 {
		rtts2 = []float64{-1}
	}
	return rtts2
}

func preTTLs(ttls1 *[]int) []float64 {
	var ttls2 []float64
	for _, ttl := range *ttls1 {
		if ttl > 0 {
			ttls2 = append(ttls2, float64(ttl))
		}
	}
	if len(ttls2) == 0 {
		ttls2 = []float64{-1}
	}
	return ttls2
}
