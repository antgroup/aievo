package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/antgroup/aievo/agent"
	"github.com/antgroup/aievo/aievo"
	"github.com/antgroup/aievo/environment"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/memory"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/tool/search"
	// "github.com/antgroup/aievo/tool/mcp"
)

type GaiaQuestion struct {
	TaskID      string `json:"task_id"`
	Question    string `json:"Question"`
	Level       int    `json:"Level"`
	FinalAnswer string `json:"Final answer"`
	FileName    string `json:"file_name"`
}
type ResultLog struct {
	ID              int     `json:"id"`
	TaskID          string  `json:"task_id"`
	Question        string  `json:"question"`
	StandardAnswer  string  `json:"standard_answer"`
	ModelOutput     string  `json:"model_output"`
	ModelThought    string  `json:"model_thought"`
	IsCorrect       bool    `json:"is_correct"`
	TotalCorrect    int     `json:"total_correct"`
	TotalCount      int     `json:"total_count"`
	RunningAccuracy float64 `json:"running_accuracy"`
	Time            string  `json:"time"`
}

func normalizeAnswer(s string) string {
	// Convert to lower case
	lower := strings.ToLower(s)
	// Remove all spaces
	noSpaces := strings.ReplaceAll(lower, " ", "")
	// Remove all punctuation except for semicolons and periods which might be part of the answer
	reg := regexp.MustCompile(`[^\w\.;]`)
	return reg.ReplaceAllString(noSpaces, "")
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

func createEvo(client llm.LLM, ts []tool.Tool) (*aievo.AIEvo, error) {
	callbackHandler := &CallbackHandler{}

	// 实例化Agents
	//
	PlanA, _ := agent.NewBaseAgent(
		agent.WithName("PlanAgent"),
		agent.WithDesc(PlanADescription),
		agent.WithPrompt(PlanAPrompt),
		agent.WithInstruction(defaultBaseInstructions),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
		agent.WithSuffix(NULLSuffix),
	)

	FileA, _ := agent.NewBaseAgent(
		agent.WithName("FileAgent"),
		agent.WithDesc(FileADescription),
		agent.WithPrompt(FileAPrompt),
		agent.WithInstruction(defaultBaseInstructions),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
		agent.WithSuffix(NULLSuffix),
	)

	//
	WebA, _ := agent.NewBaseAgent(
		agent.WithName("WebAgent"),
		agent.WithDesc(WebADescription),
		agent.WithPrompt(WebAPrompt),
		agent.WithInstruction(defaultBaseInstructions),
		agent.WithLLM(client),
		agent.WithTools(ts),
		agent.WithCallback(callbackHandler),
		agent.WithSuffix(NULLSuffix),
	)

	// WebSumA, _ := agent.NewBaseAgent(
	// 	agent.WithName("WebSummaryAgent"),
	// 	agent.WithDesc(WebSumADescription),
	// 	agent.WithPrompt(WebSumAPrompt),
	// 	agent.WithInstruction(defaultEndBaseInstructions),
	// 	agent.WithLLM(client),
	// 	agent.WithTools([]tool.Tool{search}),
	// 	agent.WithCallback(callbackHandler),
	// )

	AnswerA, _ := agent.NewBaseAgent(
		agent.WithName("AnswerAgent"),
		agent.WithDesc(AnswerADescription),
		agent.WithPrompt(AnswerAPrompt),
		agent.WithInstruction(defaultEndBaseInstructions),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
		agent.WithSuffix(NULLSuffix),
	)

	env := environment.NewEnv()
	env.Memory = memory.NewBufferMemory()

	team := make([]schema.Agent, 0)
	team = append(team, PlanA, FileA, WebA, AnswerA)

	opts := make([]aievo.Option, 0)
	opts = append(opts,
		aievo.WithTeam(team),
		aievo.WithMaxTurn(20),
		aievo.WithCallback(callbackHandler),
		aievo.WithLLM(client),
		aievo.WithEnvironment(env),
		aievo.WithTeamLeader(PlanA),
		aievo.WithSOP(workflow),
		aievo.WithUserProxy(nil),
		aievo.WithSubMode(environment.ALLSubMode),
	)

	return aievo.NewAIEvo(opts...)
}

func main() {
	// 大模型实例化
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}
	// 实例化所需要的Tools
	// 搜索引擎
	searchApiKey := os.Getenv("SERPAPI_API_KEY")
	search, _ := search.New(
		search.WithEngine("google"),
		search.WithApiKey(searchApiKey),
		search.WithTopK(5),
	)
	tools := []tool.Tool{search}

	//	tools, err := mcp.New(fmt.Sprintf(`
	//{
	//  "mcpServers": {
	//    "mcp-server-firecrawl": {
	//      "command": "npx",
	//      "args": ["-y", "firecrawl-mcp"],
	//      "env": {
	//        "FIRECRAWL_API_KEY": "%s"
	//      }
	//    }
	//  }
	//}
	//`, "fc-a31dbc4a572145faa888bd8d3f45fa71"))
	//	if err != nil {
	//		log.Fatalf("mcp register err: %+v", err)
	//	}
	//tools = append(tools, calculator.Calculator{})

	levels := []int{1}
	for _, level := range levels {
		datasetPath := fmt.Sprintf("../../../dataset/gaia/level_%d_val_filtered.json", level)
		fmt.Printf("\n################## Starting Evaluation for Level %d ##################\n", level)
		fmt.Printf("Loading dataset from: %s\n", datasetPath)

		questions, err := loadGaiaDataset(datasetPath)
		if err != nil {
			log.Printf("Failed to load GAIA dataset for level %d, skipping: %v", level, err)
			continue
		}

		var results []ResultLog
		correctCount := 0
		totalCount := 0
		timeStamp := time.Now().Format("20060102150405")
		resultsFilename := fmt.Sprintf("eval/eval_level_%d_%s.json", level, timeStamp)
		logFilename := fmt.Sprintf("eval/eval_level_%d_%s.log", level, timeStamp)
		start_time := time.Now()
		start_id := 0

		for i, q := range questions {
			if q.FileName != "" { // 先忽略需要file的问题
				continue
			}
			if i < start_id && level < 2 {
				continue
			}

			evo, err := createEvo(client, tools)
			if err != nil {
				panic(err)
			}

			fmt.Printf("\n==================Processing question ID: %d (Level %d)\n", i, level)
			gen, err := evo.Run(context.Background(), fmt.Sprintf("Question: %s", q.Question),
				llm.WithTemperature(0.6), llm.WithTopP(0.95))
			if err != nil {
				log.Printf("Error running engineer for task %s: %v", q.TaskID, err)
				// 记录错误信息到log文件
				logFile, logErr := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if logErr == nil {
					defer logFile.Close()
					logEntry := fmt.Sprintf("-----Level: %d, ID: %d, TaskID: %s\n---Question:%s\n---Error: %v\n", level, i, q.TaskID, q.Question, err)
					logFile.WriteString(logEntry)
				}
				continue
			}
			totalCount++

			// The return value 'gen' is a string, not a struct.
			fmt.Printf("Model Output Answer: %s\n", gen)

			modelOutputContent := gen
			isCorrect := false

			// Treat modelOutputContent as a plain string.
			// Remove potential "FINAL ANSWER:" prefix and other noise.
			processedOutput := strings.TrimPrefix(modelOutputContent, "FINAL ANSWER:")
			processedOutput = strings.TrimSpace(processedOutput)
			// Remove potential trailing period.
			processedOutput = strings.TrimSuffix(processedOutput, ".")
			// Also remove potential quotes around the answer
			processedOutput = strings.Trim(processedOutput, "\"")

			normalizedModelOutput := normalizeAnswer(processedOutput)
			normalizedStandardAnswer := normalizeAnswer(q.FinalAnswer)

			if normalizedModelOutput == normalizedStandardAnswer {
				isCorrect = true
				correctCount++
			}

			accuracy := 0.0
			if totalCount > 0 {
				accuracy = float64(correctCount) / float64(totalCount)
			}
			fmt.Printf("Is correct?     %t\n", isCorrect)

			results = append(results, ResultLog{
				ID:              i,
				TaskID:          q.TaskID,
				Question:        q.Question,
				StandardAnswer:  q.FinalAnswer,
				ModelOutput:     modelOutputContent,
				ModelThought:    "", // Thought is not available in the returned string.
				IsCorrect:       isCorrect,
				TotalCorrect:    correctCount,
				TotalCount:      totalCount,
				RunningAccuracy: accuracy,
				Time:            time.Since(start_time).String(),
			})

			resultsJSON, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				log.Fatalf("Failed to marshal results to JSON: %v", err)
			}

			err = os.WriteFile(resultsFilename, resultsJSON, 0644)
			if err != nil {
				log.Fatalf("Failed to write results to file: %v", err)
			}

			fmt.Printf("Current Correct Count: %d\tTotal Count: %d\n", correctCount, totalCount)
			fmt.Printf("Current Accuracy for Level %d: %.3f%%\n", level, accuracy*100)
		}

		fmt.Printf("\nEvaluation finished for Level %d. Results saved to %s\n", level, resultsFilename)
	}

	fmt.Println("\n\nAll evaluation levels are complete.")
}
