package text

import (
	"container/list"
	"strings"
)

// SensitivityDetector is a struct that contains sensitive words and output model.
// If outputModel is "AllWords", it will return all the sensitive words,
// else it will return the first sensitive word.
type SensitivityDetector struct {
	SensitiveWords []string
	Content        string
	OutputModel    string
	trie           *trie
}

func NewSensitivityDetector(sensitiveWords []string, content, outputModel string) Processor {
	trie := NewTrie()
	return &SensitivityDetector{
		SensitiveWords: sensitiveWords,
		Content:        content,
		OutputModel:    outputModel,
		trie:           trie,
	}
}

func (s SensitivityDetector) Process() (string, error) {
	for _, keyword := range s.SensitiveWords {
		s.trie.insert(keyword)
	}
	s.trie.buildFailurePointers()
	return s.trie.search(s.Content, s.OutputModel)
}
func (t *trie) insert(keyword string) {
	root := t.root
	for _, ch := range keyword {
		if _, ok := root.children[ch]; !ok {
			root.children[ch] = &node{children: make(map[rune]*node)}
		}
		root = root.children[ch]
	}
	root.isEnd = true
}

func (t *trie) buildFailurePointers() {
	queue := list.New()
	t.root.fail = nil
	queue.PushBack(t.root)

	for queue.Len() > 0 {
		current := queue.Remove(queue.Front()).(*node)

		for r, child := range current.children {
			if current == t.root {
				child.fail = t.root
			} else {
				failure := current.fail
				for failure != nil && failure.children[r] == nil {
					failure = failure.fail
				}
				if failure == nil {
					child.fail = t.root
				} else {
					child.fail = failure.children[r]
				}
			}

			if child.fail.isEnd {
				child.isEnd = true
			}

			queue.PushBack(child)
		}
	}
}

func (t *trie) search(text, mode string) (string, error) {
	currentNode := t.root
	var filterWords []string

	for i, ch := range text {
		for currentNode != nil && currentNode.children[ch] == nil {
			if currentNode == t.root {
				break
			}
			currentNode = currentNode.fail
		}

		if currentNode != nil {
			if child, ok := currentNode.children[ch]; ok {
				currentNode = child
			} else {
				currentNode = t.root
			}
		}

		if currentNode.isEnd {
			filterWords = append(filterWords, text[i-len(currentNode.children)+1:i+1])
		}
	}

	if mode == FirstSensitiveWord && len(filterWords) > 0 {
		return filterWords[0], nil
	}
	return strings.Join(filterWords, ", "), nil
}
