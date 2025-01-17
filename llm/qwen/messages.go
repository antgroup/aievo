package qwen

// Message The definition of a QWen message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AssistantMessage The definition of a QWen assistant message
type AssistantMessage struct {
	Message
	Partial bool `json:"partial"`
}
