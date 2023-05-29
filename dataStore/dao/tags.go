package dao

import "strings"

func dictedTagstring(s string) map[string]string {
	if s == "" {
		return map[string]string{}
	}
	s = strings.Replace(s, " ", "", -1)

	tagDict := make(map[string]string)
	tags := strings.Split(s, ",")
	for _, tag := range tags {
		tagPair := strings.SplitN(tag, "=", 2)
		if len(tagPair) == 2 {
			tagDict[tagPair[0]] = tagPair[1]
		}
	}
	return tagDict
}
