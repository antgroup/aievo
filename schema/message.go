package schema

import (
	"strings"
)

type Message struct {
	Type      string `json:"cate"`
	Thought   string `json:"thought"`
	Content   string `json:"content"`
	Sender    string `json:"sender"`
	Receiver  string `json:"receiver"`
	Condition string `json:"condition"`
	Token     int    `json:"token"`
	Log       string
	// control msg, to remove and update Agent
	MngInfo     *MngInfo
	AllReceiver []string
}

func (m *Message) IsEnd() bool {
	return strings.EqualFold(m.Type, MsgTypeEnd)
}

func (m *Message) IsMsg() bool {
	return strings.EqualFold(m.Type, MsgTypeMsg)
}

func (m *Message) IsCreative() bool {
	return strings.EqualFold(m.Type, MsgTypeCreative)
}

func (m *Message) IsSOP() bool {
	return strings.EqualFold(m.Type, MsgTypeSOP)
}

func (m *Message) Receivers() []string {
	receivers := make([]string, 0)
	if strings.EqualFold(m.Receiver, MsgAllReceiver) {
		receivers = m.AllReceiver
	} else if strings.Contains(m.Receiver, ",") {
		receivers = strings.Split(m.Receiver, ",")
	} else if m.Receiver != "" {
		receivers = []string{m.Receiver}
	}
	for i, receiver := range receivers {
		receivers[i] = strings.TrimSpace(receiver)
	}
	return receivers
}
