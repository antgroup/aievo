package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/antgroup/aievo/agent"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/schema"
)

type MBPPQuestion struct {
	TaskID      int      `json:"task_id"`
	Prompt      string   `json:"prompt"`
	Code        string   `json:"code"`
	TestImports []string `json:"test_imports"`
	TestList    []string `json:"test_list"`
	EntryPoint  string   `json:"entry_point"`
	Test        string   `json:"test"`
}
type ResultLog struct {
	ID       int    `json:"id"`
	TaskID   int `json:"task_id"`
	Question string `json:"question"`
	Analysis string `json:"analysis"`
}

// loadMBPPDataset loads the MBPP dataset from a JSONL file
func loadMBPPDataset(filePath string) ([]MBPPQuestion, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var questions []MBPPQuestion
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var q MBPPQuestion
		if err := json.Unmarshal(scanner.Bytes(), &q); err != nil {
			// Skip malformed lines but continue processing others
			log.Printf("skip malformed jsonl line: %v", err)
			continue
		}
		questions = append(questions, q)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return questions, nil
}

// tryResolveDataset tries multiple relative paths to find the dataset file
func tryResolveDataset(rel string) (string, error) {
	candidates := []string{
		rel,
		filepath.Join("..", "..", "..", rel),
		filepath.Join("..", "..", "..", "..", rel),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("dataset not found, tried: %v", candidates)
}

func main() {
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}

	// modes := []string{"train", "eval", "test"}
	modes := []string{"pro"}
	for _, mode := range modes {
		rel := filepath.Join("dataset", "MBPP", fmt.Sprintf("mbpp_%s.jsonl", mode))
		datasetPath, err := tryResolveDataset(rel)
		if err != nil {
			log.Printf("Failed to locate MBPP %s dataset: %v", mode, err)
			continue
		}
		fmt.Printf("Loading MBPP %s dataset from: %s\n", mode, datasetPath)

		questions, err := loadMBPPDataset(datasetPath)
		if err != nil {
			log.Printf("Failed to load MBPP dataset for %s, skipping: %v", mode, err)
			continue
		}

		var results []ResultLog
		resultsFilename := fmt.Sprintf("anal_%s.json", mode)
		startTime := time.Now()

		for i, q := range questions {
			question := q.Prompt

			fmt.Printf("\n================== Processing question ID: %d (%s)\n", i, mode)
			prompt := fmt.Sprintf(AnalysisPrompt, question)

			baseAgent, err := agent.NewBaseAgent(
				agent.WithName("AnalysisAgent"),
				agent.WithDesc("An agent that analyzes a MBPP prompt, estimates difficulty, and proposes roles."),
				agent.WithPrompt(prompt),
				agent.WithLLM(client),
				agent.WithInstruction(""),
				agent.WithSuffix(""),
			)
			if err != nil {
				log.Fatalf("failed to create agent: %v", err)
			}
			gen, err := baseAgent.Run(context.Background(), []schema.Message{
				{
					Type:    schema.MsgTypeMsg,
					Content: "",
					Sender:  "User",
				},
			}, llm.WithTemperature(0.6), llm.WithTopP(0.95))
			if err != nil {
				log.Printf("Error running agent for task %d: %v", q.TaskID, err)
				continue
			}

			// Prefer Content (JSON), fallback to Thought if empty
			var modelOutputContent string
			if len(gen.Messages) > 0 && gen.Messages[0].Content != "" {
				modelOutputContent = gen.Messages[0].Content
			} else if len(gen.Messages) > 0 {
				modelOutputContent = gen.Messages[0].Thought
			}

			fmt.Printf("Model Analysis: %s\n", modelOutputContent)

			results = append(results, ResultLog{
				ID:       i,
				TaskID:   q.TaskID,
				Question: q.Prompt,
				Analysis: modelOutputContent,
			})

			resultsJSON, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				log.Fatalf("Failed to marshal results to JSON: %v", err)
			}

			if err := os.WriteFile(resultsFilename, resultsJSON, 0644); err != nil {
				log.Fatalf("Failed to write results to file: %v", err)
			}
		}

		duration := time.Since(startTime)
		fmt.Printf("\nFinished for %s in %s. Results saved to %s\n", mode, duration, resultsFilename)
	}

	fmt.Println("\nAll modes analysis are complete.")
}
