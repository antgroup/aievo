package retriever

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuery(t *testing.T) {
	retriever := NewBgeRetriever(
		WithProviderUrl(os.Getenv("AIEVO_BGE_PROVIDER_URL")),
	)

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
