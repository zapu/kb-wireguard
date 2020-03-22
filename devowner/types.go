package devowner

type PipeMsg struct {
	ID      string      `json:"i"`
	Payload interface{} `json:"p"`
}
