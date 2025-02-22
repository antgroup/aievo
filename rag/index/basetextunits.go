package index

import (
	"context"

	"github.com/antgroup/aievo/rag/index/textsplitter"
	"github.com/pkoukk/tiktoken-go"
)

func BaseTextUnits(_ context.Context, args *WorkflowContext) error {
	opts := make([]textsplitter.Option, 0)
	if args.config.ChunkSize > 0 {
		opts = append(opts,
			textsplitter.WithChunkSize(args.config.ChunkSize))
	}
	if args.config.ChunkOverlap > 0 {
		opts = append(opts,
			textsplitter.WithChunkOverlap(args.config.ChunkOverlap))
	}
	if len(args.config.Separators) > 0 {
		opts = append(opts,
			textsplitter.WithSeparators(args.config.Separators))
	}

	splitter := textsplitter.NewMarkdownHeaderTextSplitter(
		opts...)
	tk, err := tiktoken.GetEncoding(textsplitter.DefaultTokenEncoding)
	if err != nil {
		return err
	}

	for _, document := range args.Documents {
		chunks, err := splitter.SplitText(document.Content)
		if err != nil {
			return err
		}
		for _, chunk := range chunks {
			args.TextUnits = append(args.TextUnits,
				&TextUnit{
					Id:              id(chunk),
					Text:            chunk,
					DocumentIds:     []string{document.Id},
					EntityIds:       []string{},
					RelationshipIds: []string{},
					NumToken: len(tk.Encode(chunk,
						nil, nil)),
				})
		}
	}
	return nil
}
