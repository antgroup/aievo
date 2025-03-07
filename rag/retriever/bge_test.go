package retriever

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuery(t *testing.T) {
	retriever := NewBgeRetriever()

	text := "That is a happy person"
	source := []string{
		"That is a happy dog",
		"That is a very happy person",
		"Today is a sunny day",
	}

	records, err := retriever.Query(context.Background(), text, source, 2)
	assert.Nil(t, err)

	for _, record := range records {
		fmt.Println(record)
	}
}
