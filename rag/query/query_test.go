package query

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/rag"
	"github.com/antgroup/aievo/rag/index"
	"github.com/antgroup/aievo/rag/retriever"
	db2 "github.com/antgroup/aievo/rag/storage/db"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestGormDB() (*gorm.DB, error) {
	dsn := os.Getenv("AIEVO_DSN")
	var err error
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, _ := db.DB()
	if err = sqlDB.Ping(); err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Minute * 10)
	return db, nil
}

func newTestBgeRetriever() rag.Retriever {
	bgeRetriever := retriever.NewBgeRetriever(
		retriever.WithProviderUrl(os.Getenv("AIEVO_BGE_PROVIDER_URL")),
	)
	return bgeRetriever
}

func TestIndex(t *testing.T) {
	db, err := newTestGormDB()
	assert.Nil(t, err)

	deepseek, err := openai.New(
		openai.WithBaseURL(os.Getenv("AISTUDIO_BASE_URL")),
		openai.WithToken(os.Getenv("AISTUDIO_API_KEY")),
		openai.WithModel(os.Getenv("AISTUDIO_MODEL")),
	)

	wfCtx := rag.NewWorkflowContext()
	wfCtx.Id = 2
	wfCtx.BasePath = os.Getenv("SOFARPC_DOC_PATH")

	workflow, err := index.NewWorkflow(
		index.DefaultNodes(),
		rag.WithLLM(deepseek),
		rag.WithChunkSize(2000),
		rag.WithChunkOverlap(400),
		rag.WithMaxToken(index.DefaultMaxToken),
		rag.WithMaxTurn(index.DefaultMaxTurn),
		rag.WithEntityTypes(index.DefaultEntityTypes),
		rag.WithLLMCallConcurrency(5),
		rag.WithDB(db),
	)
	assert.Nil(t, err)

	ctx := context.Background()

	err = workflow.Run(ctx, wfCtx)
}

func TestQuery(t *testing.T) {
	db, err := newTestGormDB()
	assert.Nil(t, err)

	bgeRetriever := retriever.NewBgeRetriever(
		retriever.WithProviderUrl(os.Getenv("AIEVO_BGE_PROVIDER_URL")),
	)

	deepseek, err := openai.New(
		openai.WithBaseURL(os.Getenv("AISTUDIO_BASE_URL")),
		openai.WithToken(os.Getenv("AISTUDIO_API_KEY")),
		openai.WithModel(os.Getenv("AISTUDIO_MODEL")),
	)

	wfCtx := rag.NewWorkflowContext()
	wfCtx.Id = 2
	wfCtx.BasePath = os.Getenv("SOFARPC_DOC_PATH")
	wfCtx.QueryConfig = &rag.QueryConfig{
		LLM:         deepseek,
		Retriever:   bgeRetriever,
		LLMMaxToken: 12 * 1024,
		MaxTurn:     6,
	}

	storage := db2.NewStorage(db2.WithDB(db))
	ctx := context.Background()
	err = storage.Load(ctx, wfCtx)
	assert.Nil(t, err)

	r := &RAG{
		WorkflowContext: wfCtx,
	}
	result, err := r.LocalQuery(ctx, "Why does Marley's ghost warn Scrooge that he will be visited by three spirits?")
	assert.Nil(t, err)
	fmt.Println(result)
}

func TestLLM(t *testing.T) {
	deepseek, err := openai.New(
		openai.WithBaseURL(os.Getenv("AISTUDIO_BASE_URL")),
		openai.WithToken(os.Getenv("AISTUDIO_API_KEY")),
		openai.WithModel(os.Getenv("AISTUDIO_MODEL")),
	)
	assert.Nil(t, err)

	ctx := context.Background()
	generation, err := deepseek.GenerateContent(ctx, []llm.Message{
		llm.NewUserMessage("", "你是谁?"),
	})
	assert.Nil(t, err)
	fmt.Println(generation.Content)
}
