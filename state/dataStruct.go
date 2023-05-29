package state

type MetricValue struct {
	Nid          string            `json:"nid"`
	Metric       string            `json:"metric"`
	Endpoint     string            `json:"endpoint"`
	Timestamp    int64             `json:"timestamp"`
	Step         int64             `json:"step"`
	ValueUntyped interface{}       `json:"value"`
	Value        float64           `json:"-"`
	CounterType  string            `json:"counterType"`
	Tags         string            `json:"tags"`
	TagsMap      map[string]string `json:"tagsMap"` //保留2种格式，方便后端组件使用
	Extra        string            `json:"extra"`
}

type PushResponse struct {
	Dat string `json:"dat"`
	Err string `json:"err"`
}
