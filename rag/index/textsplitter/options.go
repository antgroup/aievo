package textsplitter

import (
	"github.com/antgroup/aievo/rag"
)

// Options is a struct that contains options for a text splitter.
type Options struct {
	ChunkSize        int
	ChunkOverlap     int
	Separators       []string
	EncodingName     string
	SecondSplitter   TextSplitter
	HeadersToSplitOn []HeaderType
}

// DefaultOptions returns the default options for all text splitter.
func DefaultOptions() Options {
	return Options{
		ChunkSize:    _defaultTokenChunkSize,
		ChunkOverlap: _defaultTokenChunkOverlap,
		Separators:   []string{"\n\n", "\n", " ", ""},

		EncodingName: rag.DefaultTokenEncoding,
		HeadersToSplitOn: []HeaderType{
			{Type: "#", Name: "H1"},
			{Type: "##", Name: "H2"},
			{Type: "###", Name: "H3"},
			{Type: "####", Name: "H4"},
			{Type: "#####", Name: "H5"},
		},
	}
}

// Option is a function that can be used to set options for a text splitter.
type Option func(*Options)

// WithChunkSize sets the chunk size for a text splitter.
func WithChunkSize(chunkSize int) Option {
	return func(o *Options) {
		o.ChunkSize = chunkSize
	}
}

// WithChunkOverlap sets the chunk overlap for a text splitter.
func WithChunkOverlap(chunkOverlap int) Option {
	return func(o *Options) {
		o.ChunkOverlap = chunkOverlap
	}
}

// WithSeparators sets the separators for a text splitter.
func WithSeparators(separators []string) Option {
	return func(o *Options) {
		o.Separators = separators
	}
}

// WithEncodingName sets the encoding name for a text splitter.
func WithEncodingName(encodingName string) Option {
	return func(o *Options) {
		o.EncodingName = encodingName
	}
}

// WithSecondSplitter sets the second splitter for a text splitter.
func WithSecondSplitter(secondSplitter TextSplitter) Option {
	return func(o *Options) {
		o.SecondSplitter = secondSplitter
	}
}

// WithHeadersToSplitOn sets header to split for a md splitter.
func WithHeadersToSplitOn(headers []HeaderType) Option {
	return func(o *Options) {
		o.HeadersToSplitOn = headers
	}
}
