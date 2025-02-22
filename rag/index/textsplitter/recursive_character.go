package textsplitter

import (
	"log"
	"strings"
)

// RecursiveCharacter is a text splitter that will split texts recursively by different
// characters.
type RecursiveCharacter struct {
	Separators   []string
	ChunkSize    int
	ChunkOverlap int
}

// NewRecursiveCharacter creates a new recursive character splitter with default values. By
// default the separators used are "\n\n", "\n", " " and "". The chunk size is set to 4000
// and chunk overlap is set to 200.
func NewRecursiveCharacter(opts ...Option) RecursiveCharacter {
	options := DefaultOptions()
	for _, o := range opts {
		o(&options)
	}

	s := RecursiveCharacter{
		Separators:   options.Separators,
		ChunkSize:    options.ChunkSize,
		ChunkOverlap: options.ChunkOverlap,
	}

	return s
}

// SplitText splits a text into multiple text.
func (s RecursiveCharacter) SplitText(text string) ([]string, error) {
	finalChunks := make([]string, 0)

	// Find the appropriate separator
	separator := s.Separators[len(s.Separators)-1]
	for _, s := range s.Separators {
		if s == "" {
			separator = s
			break
		}

		if strings.Contains(text, s) {
			separator = s
			break
		}
	}

	splits := strings.Split(text, separator)
	goodSplits := make([]string, 0)

	// Merge the splits, recursively splitting larger texts.
	for _, split := range splits {
		if len(split) < s.ChunkSize || split == text {
			goodSplits = append(goodSplits, split)
			continue
		}

		if len(goodSplits) > 0 {
			mergedText := mergeSplits(goodSplits, separator, s.ChunkSize, s.ChunkOverlap)

			finalChunks = append(finalChunks, mergedText...)
			goodSplits = make([]string, 0)
		}

		otherInfo, err := s.SplitText(split)
		if err != nil {
			return nil, err
		}
		finalChunks = append(finalChunks, otherInfo...)
	}

	if len(goodSplits) > 0 {
		mergedText := mergeSplits(goodSplits, separator, s.ChunkSize, s.ChunkOverlap)
		finalChunks = append(finalChunks, mergedText...)
	}

	return finalChunks, nil
}

// joinDocs comines two documents with the separator used to split them.
func joinDocs(docs []string, separator string) string {
	return strings.TrimSpace(strings.Join(docs, separator))
}

// mergeSplits merges smaller splits into splits that are closer to the chunkSize.
func mergeSplits(splits []string, separator string, chunkSize int, chunkOverlap int) []string { //nolint:cyclop
	docs := make([]string, 0)
	currentDoc := make([]string, 0)
	total := 0

	for _, split := range splits {
		totalWithSplit := total + len(split)
		if len(currentDoc) != 0 {
			totalWithSplit += len(separator)
		}

		maybePrintWarning(total, chunkSize)
		if totalWithSplit > chunkSize && len(currentDoc) > 0 {
			doc := joinDocs(currentDoc, separator)
			if doc != "" {
				docs = append(docs, doc)
			}

			for shouldPop(chunkOverlap, chunkSize, total, len(split), len(separator), len(currentDoc)) {
				total -= len(currentDoc[0]) //nolint:gosec
				if len(currentDoc) > 1 {
					total -= len(separator)
				}
				currentDoc = currentDoc[1:] //nolint:gosec
			}
		}

		currentDoc = append(currentDoc, split)
		total += len(split)
		if len(currentDoc) > 1 {
			total += len(separator)
		}
	}

	doc := joinDocs(currentDoc, separator)
	if doc != "" {
		docs = append(docs, doc)
	}

	return docs
}

func maybePrintWarning(total, chunkSize int) {
	if total > chunkSize {
		log.Printf(
			"[WARN] created a chunk with size of %v, which is longer then the specified %v\n",
			total,
			chunkSize,
		)
	}
}

// Keep popping if:
//   - the chunk is larger then the chunk overlap
//   - or if there are any chunks and the length is long
func shouldPop(chunkOverlap, chunkSize, total, splitLen, separatorLen, currentDocLen int) bool {
	docsNeededToAddSep := 2
	if currentDocLen < docsNeededToAddSep {
		separatorLen = 0
	}

	return currentDocLen > 0 && (total > chunkOverlap || (total+splitLen+separatorLen > chunkSize && total > 0))
}
