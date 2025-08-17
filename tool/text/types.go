package text

const (
	// AllSensitiveWords return all sensitive words.
	AllSensitiveWords = "all-sensitive-words"
	// FirstSensitiveWord return the first sensitive word.
	FirstSensitiveWord = "first-sensitive-word"
)

// node represents a node in the Trie.
type node struct {
	children map[rune]*node
	isEnd    bool
	fail     *node // Failure pointer for Aho-Corasick algorithm
}

// trie Data structure for Aho-Corasick algorithm.
type trie struct {
	root *node
}

// NewTrie creates and returns a new Trie.
func NewTrie() *trie {
	return &trie{root: &node{children: make(map[rune]*node)}}
}
