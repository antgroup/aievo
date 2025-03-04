package textsplitter

import (
	"fmt"

	"github.com/pkoukk/tiktoken-go"
)

const (
	_defaultTokenChunkSize    = 2048
	_defaultTokenChunkOverlap = 200
)

// TokenSplitter is a text splitter that will split texts by tokens.
type TokenSplitter struct {
	ChunkSize    int
	ChunkOverlap int
	EncodingName string
}

func NewTokenSplitter(opts ...Option) TokenSplitter {
	options := DefaultOptions()
	for _, o := range opts {
		o(&options)
	}

	s := TokenSplitter{
		ChunkSize:    options.ChunkSize,
		ChunkOverlap: options.ChunkOverlap,
		EncodingName: options.EncodingName,
	}

	return s
}

// SplitText splits a text into multiple text.
func (s TokenSplitter) SplitText(text string) ([]string, error) {
	// Get the tokenizer
	var tk *tiktoken.Tiktoken
	var err error
	if s.EncodingName != "" {
		return nil, fmt.Errorf("tiktoken.EncodingName cannot be blank")
	}
	tk, err = tiktoken.GetEncoding(s.EncodingName)
	if err != nil {
		return nil, fmt.Errorf("tiktoken.GetEncoding: %w", err)
	}
	texts := s.splitText(text, tk)

	return texts, nil
}

func (s TokenSplitter) splitText(text string, tk *tiktoken.Tiktoken) []string {
	splits := make([]string, 0)
	inputIds := tk.Encode(text, nil, nil)

	startIdx := 0
	curIdx := len(inputIds)
	if startIdx+s.ChunkSize < curIdx {
		curIdx = startIdx + s.ChunkSize
	}
	for startIdx < len(inputIds) {
		chunkIds := inputIds[startIdx:curIdx]
		splits = append(splits, tk.Decode(chunkIds))
		startIdx += s.ChunkSize - s.ChunkOverlap
		curIdx = startIdx + s.ChunkSize
		if curIdx > len(inputIds) {
			curIdx = len(inputIds)
		}
	}
	return splits
}
