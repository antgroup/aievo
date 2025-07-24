package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/antgroup/aievo/agent"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/schema"
)


type GaiaQuestion struct {
	TaskID      string `json:"task_id"`
	Question    string `json:"Question"`
	Level       int    `json:"Level"`
	FinalAnswer string `json:"Final answer"`
	FileName    string `json:"file_name"`
}
type ResultLog struct {
	ID       int    `json:"id"`
	TaskID   string `json:"task_id"`
	Question string `json:"question"`
	Analysis string `json:"analysis"`
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

	levels := []int{1, 2, 3}
	for _, level := range levels {
		datasetPath := fmt.Sprintf("/Users/liuxiansheng/Agent/aievo/dataset/gaia/level_%d_val_new.json", level)
		fmt.Printf("Loading dataset from: %s\n", datasetPath)

		questions, err := loadGaiaDataset(datasetPath)
		if err != nil {
			log.Printf("Failed to load GAIA dataset for level %d, skipping: %v", level, err)
			continue
		}

		var results []ResultLog
		resultsFilename := fmt.Sprintf("anal_level_%d.json", level)
		start_time := time.Now()

		for i, q := range questions {

			var question string
			if q.FileName != "" {
				question = fmt.Sprintf("Question: %s\nFILENAME:%s", q.Question, q.FileName)
			} else {
				question = fmt.Sprintf("Question: %s", q.Question)
			}
			fmt.Printf("\n==================Processing question ID: %d (Level %d)\n", i, level)
			prompt := fmt.Sprintf(AnalysisPrompt,
				question,
			)

			baseAgent, err := agent.NewBaseAgent(
				agent.WithName("Agent"),
				agent.WithDesc("An agent that anlayzes given question."),
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
				log.Printf("Error running agent for task %s: %v", q.TaskID, err)
				continue
			}

			// 打印模型输出gen.Message
			fmt.Printf("Model Thought: %s\n", gen.Messages[0].Thought)
			modelOutputContent := gen.Messages[0].Thought

			results = append(results, ResultLog{
				ID:       i,
				TaskID:   q.TaskID,
				Question: q.Question,
				Analysis: modelOutputContent,
			})

			resultsJSON, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				log.Fatalf("Failed to marshal results to JSON: %v", err)
			}

			err = os.WriteFile(resultsFilename, resultsJSON, 0644)
			if err != nil {
				log.Fatalf("Failed to write results to file: %v", err)
			}

		}

		duration := time.Since(start_time)
		fmt.Printf("\nFinished for Level %d in %s. Results saved to %s\n", level, duration, resultsFilename)
	}

	fmt.Println("\n\nAll levels analysis are complete.")
}
