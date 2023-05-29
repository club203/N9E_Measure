package transmitter

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

//var mtr_s1_c1 string = "mtr -r #desip#"

// -s 包大小
// -c 多少个包
//-n 不做域名反向解析

//var mtrS1C1 string = "mtr #desip# -s 1  -c 1 -r"
var mtrS1C1 string = "mtr #desip# -s 1  -c 1 -r -n"

func MTR(ip string) []string {
	cmd := strings.Replace(mtrS1C1, "#desip#", ip, -1)
	output := ExecShell(cmd)
	newOutput := outputFormat(output)
	//fmt.Println()
	return newOutput
}

func outputFormat(mtr_output string) []string {
	// sentAllPktTs: Sun Mar 28 14:58:22 2021
	// HOST: localhost                   Loss%   Snt   Last   Avg  Best  Wrst StDev
	//  1.|-- 103.228.162.73             0.0%     1    0.2   0.2   0.2   0.2   0.0
	//  2.|-- 183.6.205.245              0.0%     1    1.4   1.4   1.4   1.4   0.0
	//  3.|-- ???                       100.0     1    0.0   0.0   0.0   0.0   0.0
	//  4.|-- 116.31.110.253             0.0%     1    2.1   2.1   2.1   2.1   0.0
	//  5.|-- 113.96.255.26              0.0%     1    9.0   9.0   9.0   9.0   0.0
	//  6.|-- 202.97.91.134              0.0%     1   15.9  15.9  15.9  15.9   0.0
	//  7.|-- 202.97.91.194              0.0%     1   13.3  13.3  13.3  13.3   0.0
	//  8.|-- 202.97.98.181              0.0%     1  173.2 173.2 173.2 173.2   0.0
	//  9.|-- 202.97.83.230              0.0%     1  181.2 181.2 181.2 181.2   0.0
	// 10.|-- 218.30.54.214              0.0%     1  196.7 196.7 196.7 196.7   0.0
	// 11.|-- 1.1.1.225                  0.0%     1  209.3 209.3 209.3 209.3   0.0

	// "1.0.79.1":["10:58.138.118.158:246:168.374","11:202.232.8.34:246:176.777","12:210.166.33.169:245:172.997","13:210.166.40.50:244:173.282","14:1.0.79.1:115:180.802"]
	trace := strings.Split(strings.Trim(mtr_output, " "), ".|--")

	if len(trace) > 0 && (strings.Contains(trace[0], "HOST") || strings.Contains(trace[0], "sentAllPktTs")) {
		trace = trace[1:]
	}
	if len(trace) > 0 && (strings.Contains(trace[0], "HOST") || strings.Contains(trace[0], "sentAllPktTs")) {
		trace = trace[1:]
	}

	newTrace := make([]string, len(trace))
	for i, singleTrace := range trace {
		index := i + 1
		if strings.Contains(singleTrace, "??") {
			//"1.0.63.1":["2:*","3:*","4:*","5:*.....
			newTrace[i] = fmt.Sprintf("%d:*", index)
			continue
		}
		// 直接按照空白字符分割
		//fmt.Println(singleTrace)
		results := strings.Fields(singleTrace)
		// index:  		ip	 :ttl?:rtt
		// "10:58.138.118.158:246:168.374"
		newTrace[i] = fmt.Sprintf("%d:%s:-1:%s", index, results[0], results[3])
	}

	//newOutput := strings.Join(newTrace, ",")
	return newTrace

}

func ExecShell(s string) string {
	//start_ts := time.Now().UnixNano()
	cmd := exec.Command("/bin/bash", "-c", s)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	//var output string = out.String() + "@" + s
	var output = out.String()
	//fmt.Println(output)
	//end_ts := time.Now().UnixNano()
	//delta := (end_ts - start_ts) / 1e6
	//fmt.Printf("消耗 %s ms\n\n", fmt.Sprintf("%d", delta))
	if err != nil {
		log.Fatal(err)
		return ""
	} else {
		return output
	}
}
