package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/antgroup/aievo/agent"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"

	// "github.com/antgroup/aievo/tool/bash"
	// "github.com/antgroup/aievo/tool/file"
	"github.com/antgroup/aievo/tool/search"
)

type GaiaQuestion struct {
	TaskID      string `json:"task_id"`
	Question    string `json:"Question"`
	Level       int    `json:"Level"`
	FinalAnswer string `json:"Final answer"`
	FileName    string `json:"file_name"`
}

type ModelOutput struct {
	Thought  string `json:"thought"`
	Cate     string `json:"cate"`
	Receiver string `json:"receiver"`
	Content  string `json:"content"`
}

type ResultLog struct {
	TaskID          string  `json:"task_id"`
	Question        string  `json:"question"`
	StandardAnswer  string  `json:"standard_answer"`
	ModelOutput     string  `json:"model_output"`
	ModelThought    string  `json:"model_thought"`
	IsCorrect       bool    `json:"is_correct"`
	RunningAccuracy float64 `json:"running_accuracy"`
}

func loadGaiaDataset(filePath string) ([]GaiaQuestion, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var questions []GaiaQuestion
	err = json.Unmarshal(file, &questions)
	if err != nil {
		return nil, err
	}

	return questions, nil
}

func main() {
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}
	workspace, _ := os.Getwd()
	workspace = filepath.Join(workspace,
		"examples", "single-agent-example", "GAIA", "workspace")
	// 文件创建 文件读取 文件修改 文件删除 文件重命名
	// 文件夹创建 文件夹读取 文件夹删除 文件夹重命名
	// fileTools, _ := file.GetFileRelatedTools(workspace)  // can be nil

	callbackHandler := &CallbackHandler{}

	engineerTools := make([]tool.Tool, 0)
	// engineerTools = append(engineerTools, fileTools...)
	searchApiKey := os.Getenv("SERPAPI_API_KEY")
	search, _ := search.New(
		search.WithEngine("google"),
		search.WithApiKey(searchApiKey),
		search.WithTopK(3),
	)

	engineerTools = append(engineerTools, search)

	engineer, err := agent.NewBaseAgent(
		agent.WithName("engineer"),
		agent.WithDesc(EngineerDescription),
		agent.WithPrompt(EngineerPrompt),
		agent.WithInstruction(SingleAgentInstructions),
		agent.WithVars("sop", Workflow),
		agent.WithVars("workspace", workspace),
		agent.WithTools(engineerTools),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
	)
	if err != nil {
		log.Fatal(err)
	}

	for level := 1; level <= 3; level++ {
		datasetPath := fmt.Sprintf("/Users/liuxiansheng/Agent/aievo/dataset/gaia/level_%d_val_filtered.json", level)
		fmt.Printf("\n\n\n################## Starting Evaluation for Level %d ##################\n", level)
		fmt.Printf("Loading dataset from: %s\n", datasetPath)

		questions, err := loadGaiaDataset(datasetPath)
		if err != nil {
			log.Printf("Failed to load GAIA dataset for level %d, skipping: %v", level, err)
			continue
		}

		var results []ResultLog
		correctCount := 0
		totalCount := 0
		resultsFilename := fmt.Sprintf("evaluation_results_level%d_%s.json", level, time.Now().Format("20060102150405"))

		for i, q := range questions {
			if q.FileName != "" {
				continue
			}

			totalCount++
			fmt.Printf("\n\n\n==================Processing question ID: %d (Level %d)\n", i, level)
			gen, err := engineer.Run(context.Background(), []schema.Message{
				{
					Type:     schema.MsgTypeMsg,
					Content:  fmt.Sprintf("Question: %s", q.Question),
					Sender:   "User",
					Receiver: "engineer",
				},
			}, llm.WithTemperature(0.6), llm.WithTopP(0.95))
			if err != nil {
				log.Printf("Error running engineer for task %s: %v", q.TaskID, err)
				continue
			}

			modelOutputContent := gen.Messages[0].Content
			isCorrect := false

			// Treat modelOutputContent as a plain string.
			// Remove potential "FINAL ANSWER:" prefix and other noise.
			processedOutput := strings.TrimPrefix(modelOutputContent, "FINAL ANSWER:")
			processedOutput = strings.TrimSpace(processedOutput)
			// Remove potential trailing period.
			processedOutput = strings.TrimSuffix(processedOutput, ".")
			// Also remove potential quotes around the answer
			processedOutput = strings.Trim(processedOutput, "\"")

			if strings.EqualFold(processedOutput, q.FinalAnswer) {
				isCorrect = true
				correctCount++
			}

			accuracy := 0.0
			if totalCount > 0 {
				accuracy = float64(correctCount) / float64(totalCount)
			}

			results = append(results, ResultLog{
				TaskID:          q.TaskID,
				Question:        q.Question,
				StandardAnswer:  q.FinalAnswer,
				ModelOutput:     modelOutputContent,
				ModelThought:    gen.Messages[0].Thought,
				IsCorrect:       isCorrect,
				RunningAccuracy: accuracy,
			})

			resultsJSON, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				log.Fatalf("Failed to marshal results to JSON: %v", err)
			}

			err = os.WriteFile(resultsFilename, resultsJSON, 0644)
			if err != nil {
				log.Fatalf("Failed to write results to file: %v", err)
			}

			fmt.Printf("Current Accuracy for Level %d: %.3f%%\n", level, accuracy*100)
		}

		fmt.Printf("\nEvaluation finished for Level %d. Results saved to %s\n", level, resultsFilename)
	}

	fmt.Println("\n\nAll evaluation levels are complete.")
}
