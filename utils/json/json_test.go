package json

import (
	"fmt"
	"testing"
)

type Message struct {
	Type     string `json:"type"`
	Thought  string `json:"thought"`
	Msg      string `json:"msg"`
	Receiver string `json:"receiver"`
	Content  string `json:"content"`
}

var testJson1 = `
{
    "type": "end",
    "thought": "thought",
    "msg": "## title
### background
this is background
",
    "receiver": "test"
}
`

var testJson2 = `
[
{
    "type": "end",
    "thought": "thought",
    "msg": "## title
### background
this is background
",
    "receiver": "test"
},
{
    "type": "end",
    "thought": "thought",
    "msg": "## title
### background
this is background
",
    "receiver": "test"
}
]
`

var testJson3 = ""

var testJson4 = `{
  "thought": "here is what I thought",
  "cate": "MSG",
  "receiver": "test",
  "content": "hello 
world
- 1
- 2
"}`

var testJson5 = `{
  "content": "{
	\"type\": \"end\"}"
}`

var testJson6 = `{\n\t\"thought\": \"这是一个简单的数学乘法问题，我可以使用calculator工具来解决这个问题。\",\n\t\"action\": \"calculator\",\n\t\"input\": {\"param\": \"20 * 30\"}\n}`

func TestDecode1(t *testing.T) {
	var msg Message
	err := Unmarshal([]byte(testJson1), &msg)
	if err != nil {
		t.Errorf("Decode failed: %v", err)
		return
	}
	t.Logf("Decode success: %v", msg)
}

func TestDecode2(t *testing.T) {
	msgs := make([]Message, 0)
	err := Unmarshal([]byte(testJson2), &msgs)
	if err != nil {
		t.Errorf("Decode failed: %v", err)
		return
	}
	t.Logf("Decode success: %v", msgs)
}

func TestDecode3(t *testing.T) {
	var msgs []Message
	err := Unmarshal([]byte(testJson3), &msgs)
	if err != nil {
		t.Errorf("Decode failed: %v", err)
		return
	}
	t.Logf("Decode success: %v", msgs)
}

func TestDecode4(t *testing.T) {
	msgs := make([]*Message, 0, 2)
	msg := `
{
"type": "end",
"msg": "hello"}`
	err := Unmarshal([]byte(msg), &msgs)
	if err != nil {
		fmt.Print(err)
	}
}

func TestDecode5(t *testing.T) {
	msgs := &Message{}
	// testJson4 = strings.Replace(testJson4, `\"`, `'`, -1)
	err := Unmarshal([]byte(testJson4), &msgs)
	if err != nil {
		fmt.Print(err)
	}
	fmt.Println(msgs)
}

func TestDecode6(t *testing.T) {
	msgs := &Message{}
	// testJson4 = strings.Replace(testJson4, `\"`, `'`, -1)
	err := Unmarshal([]byte(testJson5), &msgs)
	if err != nil {
		fmt.Print(err)
	}
	fmt.Println(msgs)
}

func TestDecode7(t *testing.T) {
	msgs := &Message{}
	// testJson4 = strings.Replace(testJson4, `\"`, `'`, -1)
	err := Unmarshal([]byte(testJson6), &msgs)
	if err != nil {
		fmt.Print(err)
	}
	fmt.Println(msgs)
}
