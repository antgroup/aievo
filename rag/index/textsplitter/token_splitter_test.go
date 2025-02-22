package textsplitter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenSplitter(t *testing.T) {
	t.Parallel()
	type testCase struct {
		text         string
		chunkOverlap int
		chunkSize    int
		expectedDocs []string
	}
	//nolint:dupword
	testCases := []testCase{
		{
			text:         "Hi.\nI'm Harrison.\n\nHow?\na\nb",
			chunkOverlap: 1,
			chunkSize:    20,
			expectedDocs: []string{
				"Hi.\nI'm Harrison.\n\nHow?\na\nb",
			},
		},
		{
			text:         "Hi.\nI'm Harrison.\n\nHow?\na\nbHi.\nI'm Harrison.\n\nHow?\na\nb",
			chunkOverlap: 1,
			chunkSize:    40,
			expectedDocs: []string{
				"Hi.\nI'm Harrison.\n\nHow?\na\nbHi.\nI'm Harrison.\n\nHow?\na\nb",
			},
		},
		{
			text:         "name: Harrison\nage: 30",
			chunkOverlap: 1,
			chunkSize:    40,
			expectedDocs: []string{
				"name: Harrison\nage: 30",
			},
		},
		{
			text: `name: Harrison
age: 30

name: Joe
age: 32`,
			chunkOverlap: 1,
			chunkSize:    40,
			expectedDocs: []string{
				"name: Harrison\nage: 30\n\nname: Joe\nage: 32",
			},
		},
		{
			text: `Hi.
I'm Harrison.

How? Are? You?
Okay then f f f f.
This is a weird text to write, but gotta test the splittingggg some how.

Bye!

-H.`,
			chunkOverlap: 1,
			chunkSize:    10,
			expectedDocs: []string{
				"Hi.\nI'm Harrison.\n\nHow? Are?",
				"? You?\nOkay then f f f f.\n",
				".\nThis is a weird text to write, but",
				" but gotta test the splittingggg some how.\n\n",
				".\n\nBye!\n\n-H.",
			},
		},
	}
	splitter := NewTokenSplitter()
	for _, tc := range testCases {
		splitter.ChunkOverlap = tc.chunkOverlap
		splitter.ChunkSize = tc.chunkSize

		text, err := splitter.SplitText(tc.text)
		// docs, err := CreateDocuments(splitter, []string{tc.text}, nil)
		assert.NoError(t, err)
		assert.Equal(t, tc.expectedDocs, text)
	}
}
