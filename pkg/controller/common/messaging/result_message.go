package messaging

type ResultMessage struct {
	Sender            string            `json:"sender"`
	LocalOffloading   []LocalOffloading `json:"local-offloading"`
	OverallOffloading OverallOffloading `json:"overall-offloading"`
	Timestamp         float64           `json:"timestamp"`
}

type LocalOffloading struct {
	Name     string         `json:"name"`
	AppName  string         `json:"app_name"`
	Function FunctionResult `json:"function"`
}

type OverallOffloading struct {
}

type FunctionResult struct {
	Name      string          `json:"name"`
	Resources ResourcesResult `json:"resources"`
}

type ResourcesResult struct {
	Memory float64 `json:"memory"`
	Cpu    float64 `json:"cpu"`
}

func NewResultMessage(sender string, localOffloading []LocalOffloading, overallOffloading OverallOffloading, timestamp float64) *ResultMessage {
	message := ResultMessage{sender, localOffloading, overallOffloading, timestamp}
	return &message
}
