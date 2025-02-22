package textsplitter

import (
	"fmt"
	"testing"

	"github.com/antgroup/aievo/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownHeaderTextSplitter_SplitDocuments(t *testing.T) {
	content := `某君昆仲，今隐其名，皆余昔日在中学时良友；分隔多年，消息渐阙。日前偶闻其一大病；适归故乡，迂道往访，则仅晤一人，言病者其弟也。劳君远道来视，然已早愈，赴某地候补⑵矣。因大笑，出示日记二册，谓可见当日病状，不妨献诸旧友。持归阅一过，知所患盖“迫害狂”之类。语颇错杂无伦次，又多荒唐之言；亦不著月日，惟墨色字体不一，知非一时所书。间亦有略具联络者，今撮录一篇，以供医家研究。记中语误，一字不易；惟人名虽皆村人，不为世间所知，无关大体，然亦悉易去。至于书名，则本人愈后所题，不复改也。七年四月二日识。

　　一

　　今天晚上，很好的月光。

　　我不见他，已是三十多年；今天见了，精神分外爽快。才知道以前的三十多年，全是发昏；然而须十分小心。不然，那赵家的狗，何以看我两眼呢？

　　我怕得有理。

　　二

　　今天全没月光，我知道不妙。早上小心出门，赵贵翁的眼色便怪：似乎怕我，似乎想害我。还有七八个人，交头接耳的议论我，张着嘴，对我笑了一笑；我便从头直冷到脚根，晓得他们布置，都已妥当了。

　　我可不怕，仍旧走我的路。前面一伙小孩子，也在那里议论我；眼色也同赵贵翁一样，脸色也铁青。我想我同小孩子有什么仇，他也这样。忍不住大声说，“你告诉我！”他们可就跑了。

　　我想：我同赵贵翁有什么仇，同路上的人又有什么仇；只有廿年以前，把古久先生的陈年流水簿子⑶，踹了一脚，古久先生很不高兴。赵贵翁虽然不认识他，一定也听到风声，代抱不平；约定路上的人，同我作冤对。但是小孩子呢？那时候，他们还没有出世，何以今天也睁着怪眼睛，似乎怕我，似乎想害我。这真教我怕，教我纳罕而且伤心。

　　我明白了。这是他们娘老子教的！`

	splitter := NewMarkdownHeaderTextSplitter(WithHeadersToSplitOn(
		[]HeaderType{
			{Type: "#", Name: "H1"},
			{Type: "##", Name: "H2"},
			{Type: "###", Name: "H3"},
		}), WithChunkOverlap(20),
		WithChunkSize(512))
	documents, err := splitter.SplitDocuments(content)
	fmt.Println(documents, err)
}

func TestMarkdownHeaderSplitter(t *testing.T) {
	type testCase struct {
		markdown     string
		expectedDocs []schema.Document
	}

	splitter := NewMarkdownHeaderTextSplitter(WithHeadersToSplitOn(
		[]HeaderType{
			{Type: "#", Name: "H1"},
			{Type: "##", Name: "H2"},
			{Type: "###", Name: "H3"},
		}), WithChunkOverlap(20),
		WithChunkSize(512))

	testCases := []testCase{
		{
			markdown: `
### This is a header

- This is a list item of bullet type.
- This is another list item.

 *Everything* is going according to **plan**.
`,
			expectedDocs: []schema.Document{
				{
					PageContent: `- This is a list item of bullet type.
- This is another list item.  
*Everything* is going according to **plan**.`,
					Metadata: map[string]any{
						"H3": "This is a header",
					},
				},
			},
		},
		{
			markdown: "example code:\n```go\nfunc main() {}\n```",
			expectedDocs: []schema.Document{
				{PageContent: "example code:\n```go\nfunc main() {}\n```", Metadata: map[string]any{}},
			},
		},
	}

	for _, tc := range testCases {
		docs, err := splitter.SplitDocuments(tc.markdown)
		require.NoError(t, err)

		assert.Equal(t, tc.expectedDocs, docs)
	}

}

func TestMarkdownHeaderSplitterText(t *testing.T) {
	type testCase struct {
		markdown     string
		expectedDocs []string
	}

	splitter := NewMarkdownHeaderTextSplitter(WithHeadersToSplitOn(
		[]HeaderType{
			{Type: "#", Name: "H1"},
			{Type: "##", Name: "H2"},
			{Type: "###", Name: "H3"},
		}), WithChunkOverlap(20),
		WithChunkSize(512))

	testCases := []testCase{
		{
			markdown: `
### This is a header

- This is a list item of bullet type.
- This is another list item.

 *Everything* is going according to **plan**.
`,
			expectedDocs: []string{
				`### This is a header
- This is a list item of bullet type.
- This is another list item.  
*Everything* is going according to **plan**.`,
			},
		},

		{
			markdown: "example code:\n```go\nfunc main() {}\n```",
			expectedDocs: []string{
				"example code:\n```go\nfunc main() {}\n```",
			},
		},
	}

	for _, tc := range testCases {
		text, err := splitter.SplitText(tc.markdown)
		require.NoError(t, err)
		assert.Equal(t, tc.expectedDocs, text)
	}

}
