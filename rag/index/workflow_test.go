package index

import (
	"context"
	"fmt"
	"testing"
)

func TestBaseDocuments(t *testing.T) {
	args := &WorkflowContext{basepath: "/Users/tyloafer/WorkPlace/src/github.com/antgroup/aievo/rag"}
	err := BaseDocuments(context.Background(),
		args)
	if err != nil {
		panic(err)
	}
	fmt.Println(args.Documents)
}
