package measureChange

type MetaValue struct {
	Endpoint   string            `json:"endpoint"`
	Metric     string            `json:"metric"`
	Value      interface{}       `json:"value"`
	Step       string            `json:"step"`
	Type       string            `json:"counterType"`
	Tags       map[string]string `json:"tags"`
	Timestamp  string            `json:"timestamp"`
	SourceName string
	SourceIp   string
	DstIp      string
}
