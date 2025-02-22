package textsplitter

import (
	"sort"
	"strings"

	"github.com/antgroup/aievo/schema"
)

type HeaderType struct {
	level int
	Type  string
	Name  string
}

type LineType struct {
	Content  string
	Metadata map[string]any
}

type MarkdownHeaderTextSplitter struct {
	HeadersToSplitOn []HeaderType
	SecondSplitter   TextSplitter
}

var _ TextSplitter = (*MarkdownHeaderTextSplitter)(nil)

func NewMarkdownHeaderTextSplitter(opts ...Option) *MarkdownHeaderTextSplitter {
	options := DefaultOptions()

	for _, o := range opts {
		o(&options)
	}
	splitter := &MarkdownHeaderTextSplitter{
		HeadersToSplitOn: options.HeadersToSplitOn,
	}

	// modify HeadersToSplitOn level
	for i := range splitter.HeadersToSplitOn {
		splitter.HeadersToSplitOn[i].level = strings.Count(
			splitter.HeadersToSplitOn[i].Type, "#")
	}

	// Sort headers to split on by length, in descending order
	sort.Slice(splitter.HeadersToSplitOn, func(i, j int) bool {
		return len(splitter.HeadersToSplitOn[i].Type) >
			len(splitter.HeadersToSplitOn[j].Type)
	})

	// set default second splitter for content
	if splitter.SecondSplitter == nil {
		splitter.SecondSplitter = NewRecursiveCharacter(
			WithChunkSize(options.ChunkSize),
			WithChunkOverlap(options.ChunkOverlap),
			WithSeparators([]string{
				"\n\n", // new line
				"\n",   // new line
				" ",    // space
				",",    // space
				"，",    // space
				"。",    // space
				".",    // space
				"!",    // space
				"！",    // space
				";",    // space
				"；",    // space
			}),
		)
	}

	return splitter
}

func (m *MarkdownHeaderTextSplitter) AggregateLinesToChunks(lines []LineType) ([]schema.Document, error) {
	var aggregatedChunks []LineType

	for _, line := range lines {
		if len(aggregatedChunks) > 0 && compareMetadata(aggregatedChunks[len(aggregatedChunks)-1].Metadata, line.Metadata) {
			aggregatedChunks[len(aggregatedChunks)-1].Content += "  \n" + line.Content
		} else {
			aggregatedChunks = append(aggregatedChunks, line)
		}
	}

	var documents []schema.Document
	for _, chunk := range aggregatedChunks {
		// base on SecondSplitter to split again
		text, err := m.SecondSplitter.SplitText(chunk.Content)
		if err != nil {
			return nil, err
		}
		for _, content := range text {
			documents = append(documents, schema.Document{
				PageContent: content,
				Metadata:    chunk.Metadata,
			})
		}
	}

	return documents, nil
}

func compareMetadata(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for key, valueA := range a {
		if valueB, ok := b[key]; !ok || valueA != valueB {
			return false
		}
	}
	return true
}

func (m *MarkdownHeaderTextSplitter) SplitText(text string) ([]string, error) {
	docs, err := m.SplitDocuments(text)
	if err != nil {
		return nil, err
	}
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		metadata := ""
		for i := len(m.HeadersToSplitOn) - 1; i >= 0; i-- {
			if _, ok := doc.Metadata[m.HeadersToSplitOn[i].Name]; ok {
				metadata += strings.Repeat("#", m.HeadersToSplitOn[i].level) + " " +
					doc.Metadata[m.HeadersToSplitOn[i].Name].(string) + "\n"
			}
		}
		texts = append(texts, metadata+doc.PageContent)
	}
	return texts, err
}

func (m *MarkdownHeaderTextSplitter) SplitDocuments(text string) ([]schema.Document, error) {
	lines := strings.Split(text, "\n")
	var linesWithMetadata []LineType
	var currentContent []string
	currentMetadata := make(map[string]any)
	var headerStack []HeaderType
	initialMetadata := make(map[string]any)

	inCodeBlock := false
	openingFence := ""
	var foundHeader bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		foundHeader = false

		if !inCodeBlock {
			if strings.HasPrefix(line, "```") || strings.Count(line, "```") == 1 {
				inCodeBlock = true
				openingFence = "```"
			} else if strings.HasPrefix(line, "~~~") || strings.Count(line, "~~~") == 1 {
				inCodeBlock = true
				openingFence = "~~~"
			}
		} else {
			if strings.HasPrefix(line, openingFence) {
				inCodeBlock = false
				openingFence = ""
			}
		}
		if inCodeBlock {
			currentContent = append(currentContent, line)
			continue
		}

		for _, header := range m.HeadersToSplitOn {
			typ := header.Type
			name := header.Name

			if strings.HasPrefix(line, typ) &&
				(len(line) == len(typ) || strings.Index(line, " ") == len(typ)) {

				// modify name to actual title
				if name != "" {
					for len(headerStack) > 0 && headerStack[len(headerStack)-1].level >= header.level {
						poppedHeader := headerStack[len(headerStack)-1]
						headerStack = headerStack[:len(headerStack)-1]

						if _, exists := initialMetadata[poppedHeader.Type]; exists {
							delete(initialMetadata, poppedHeader.Type)
						}
					}

					modifyHeader := HeaderType{
						level: header.level,
						Type:  name,
						Name:  strings.TrimSpace(line[len(typ):]),
					}
					headerStack = append(headerStack, modifyHeader)
					initialMetadata[name] = modifyHeader.Name
				}

				if len(currentContent) > 0 {
					linesWithMetadata = append(linesWithMetadata, LineType{
						Content:  strings.Join(currentContent, "\n"),
						Metadata: copyMap(currentMetadata),
					})
					currentContent = currentContent[0:0]
				}
				foundHeader = true
				break
			}

		}

		if !foundHeader {
			if len(line) > 0 {
				currentContent = append(currentContent, line)
			} else if len(currentContent) > 0 {
				linesWithMetadata = append(linesWithMetadata, LineType{
					Content:  strings.Join(currentContent, "\n"),
					Metadata: copyMap(currentMetadata),
				})
				currentContent = currentContent[0:0]
			}
		}

		currentMetadata = copyMap(initialMetadata)
	}

	if len(currentContent) > 0 {
		linesWithMetadata = append(linesWithMetadata, LineType{
			Content:  strings.Join(currentContent, "\n"),
			Metadata: currentMetadata,
		})
	}

	return m.AggregateLinesToChunks(linesWithMetadata)
}

func copyMap(original map[string]any) map[string]any {
	copied := make(map[string]any)
	for key, value := range original {
		copied[key] = value
	}
	return copied
}
