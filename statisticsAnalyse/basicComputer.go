package statisticsAnalyse

import "math"

func average(data *[]float64) float64 {
	sum := 0.0

	for _, v := range *data {
		sum += v
	}

	return sum / float64(len(*data))
}
func variance(data *[]float64, avg float64) float64 {
	vary := 0.0

	for _, v := range *data {
		delta := v - avg
		vary += delta * delta
	}

	return vary / float64(len(*data))
}
func jitter(data *[]float64, avg float64) []float64 {
	var jitters []float64
	for _, v := range *data {
		delta := math.Abs(v - avg)
		jitters = append(jitters, delta)
	}
	return jitters
}

func quantile(data *[]float64) (float64, float64, float64) {
	n := len(*data)
	loc := []float64{0.25, 0.5, 0.75}
	result := make([]float64, 3)
	for i, f := range loc {
		index := f * float64(n-1)
		lhs := int(index)
		delta := index - float64(lhs)

		if n == 0 {
			return 0.0, 0.0, 0.0
		}

		if lhs == n-1 {
			result[i] = float64((*data)[n-1])
		} else {
			result[i] = (1-delta)*(*data)[lhs] + delta*(*data)[lhs+1]
		}
	}

	return result[0], result[1], result[2]
}
