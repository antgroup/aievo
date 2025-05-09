package index

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/rag"
	"github.com/antgroup/aievo/utils/ratelimit"
)

var tbOnce sync.Once
var cacheOnce sync.Once
var tb *ratelimit.TokenBucket

func CallModel(ctx context.Context, wfCtx *rag.WorkflowContext, messages []llm.Message, useCache bool) (*llm.Generation, error) {
	tbOnce.Do(func() {
		tb = ratelimit.NewTokenBucket(3, 3)
	})

	if useCache {
		cacheOnce.Do(func() {
			if wfCtx.CacheDir != "" {
				if _, err := os.Stat(wfCtx.CacheDir); os.IsNotExist(err) {
					err := os.MkdirAll(wfCtx.CacheDir, 0755)
					if err != nil {
						fmt.Printf("Failed to create cache directory: %v\n", err)
					}
				}
			}
		})
	}

	if wfCtx.CacheDir != "" && useCache {
		cacheKey := generateCacheKey(messages)
		cacheFilePath := filepath.Join(wfCtx.CacheDir, cacheKey)
		if _, err := os.Stat(cacheFilePath); err == nil {
			cachedResult, err := readFromCache(cacheFilePath)
			if err == nil && cachedResult != nil && cachedResult.Content != "" {
				return cachedResult, nil
			}
			_ = deleteCache(cacheFilePath)
		}
	}

	if !hitCache(wfCtx, messages) || !useCache {
		if err := tb.Wait(ctx); err != nil {
			return nil, fmt.Errorf("Failed to wait for token: %v\n", err)
		}
	}

	result, err := wfCtx.Config.LLM.GenerateContent(ctx, messages,
		llm.WithTemperature(0.1))
	if err != nil || result.Content == "" {
		return nil, fmt.Errorf("call model error")
	}

	if wfCtx.CacheDir != "" {
		cacheKey := generateCacheKey(messages)
		cacheFilePath := filepath.Join(wfCtx.CacheDir, cacheKey)
		err := writeToCache(cacheFilePath, result)
		if err != nil {
			fmt.Printf("Failed to write to cache: %v\n", err)
		}
	}
	return result, err
}

func hitCache(wfCtx *rag.WorkflowContext, messages []llm.Message) bool {
	if wfCtx.CacheDir != "" {
		cacheKey := generateCacheKey(messages)
		cacheFilePath := filepath.Join(wfCtx.CacheDir, cacheKey)
		if _, err := os.Stat(cacheFilePath); err == nil {
			return true
		}
	}
	return false
}

func generateCacheKey(messages []llm.Message) string {
	text := ""
	for _, message := range messages {
		text += message.Content
	}
	return fmt.Sprintf("%x", md5.Sum([]byte(text)))
}

func readFromCache(cacheFilePath string) (*llm.Generation, error) {
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return nil, err
	}
	var result llm.Generation
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func writeToCache(cacheFilePath string, result *llm.Generation) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFilePath, data, 0644)
}

func deleteCache(cacheFilePath string) error {
	return os.Remove(cacheFilePath)
}
