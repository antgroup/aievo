package index

import (
	"context"

	"github.com/antgroup/aievo/rag"
	"github.com/antgroup/aievo/rag/index/textsplitter"
	"github.com/pkoukk/tiktoken-go"
)

func BaseTextUnits(_ context.Context, args *rag.WorkflowContext) error {
	opts := make([]textsplitter.Option, 0)
	if args.Config.ChunkSize > 0 {
		opts = append(opts,
			textsplitter.WithChunkSize(args.Config.ChunkSize))
	}
	if args.Config.ChunkOverlap > 0 {
		opts = append(opts,
			textsplitter.WithChunkOverlap(args.Config.ChunkOverlap))
	}
	if len(args.Config.Separators) > 0 {
		opts = append(opts,
			textsplitter.WithSeparators(args.Config.Separators))
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
				&rag.TextUnit{
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
