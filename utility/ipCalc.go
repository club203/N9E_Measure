package utility

import (
	"math/big"
	"net"
)

func inetAtoN(ip string) int64 {
	ret := big.NewInt(0)
	ret.SetBytes(net.ParseIP(ip).To4())
	return ret.Int64()
}

func CalcIPNum(targets []string) int64 {
	sum := int64(0)
	for i := 0; i < len(targets); i += 2 {
		start := inetAtoN(targets[i])
		end := inetAtoN(targets[i+1])
		sum += end - start
	}
	// eg. [0.0.0.0, 0.0.0.0] means 0.0.0.0 - 0.0.0.0 = 0, but it's closed interval, so plus 254
	return sum + 254
}
