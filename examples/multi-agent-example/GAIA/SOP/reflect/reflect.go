package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/antgroup/aievo/agent"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/schema"
)

// Structs for train.json
type GaiaQuestion struct {
	TaskID            string            `json:"task_id"`
	Question          string            `json:"Question"`
	Level             int               `json:"Level"`
	FinalAnswer       string            `json:"Final answer"`
	FileName          string            `json:"file_name"`
	AnnotatorMetadata AnnotatorMetadata `json:"Annotator Metadata"`
}

type AnnotatorMetadata struct {
	Steps string `json:"Steps"`
}

// Structs for eval log
type ResultLog struct {
	ID                   int              `json:"id"`
	TaskID               string           `json:"task_id"`
	IsCorrect            bool             `json:"is_correct"`
	CommunicationHistory []schema.Message `json:"communication_history"`
}

// Struct for SOP file
type SOPFile struct {
	Question string `json:"question"`
	Analysis string `json:"analysis"`
	SOPs     []SOP  `json:"sops"`
}

type SOP struct {
	Team    []string      `json:"team"`
	SOP     string        `json:"sop"`
	Details []AgentDetail `json:"details"`
}

type AgentDetail struct {
	Name           string   `json:"name"`
	Responsibility string   `json:"responsibility"`
	Instruction    string   `json:"instruction"`
	Tools          []string `json:"tools"`
}

// loadFile unmarshals a JSON file into the given interface
func loadFile(path string, v interface{}) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}
	if err := json.Unmarshal(bytes, v); err != nil {
		return fmt.Errorf("failed to unmarshal file %s: %w", path, err)
	}
	return nil
}

func performReflection(client llm.LLM, sopContent string, history []schema.Message, question GaiaQuestion, outputPath string) error {
	log.Printf("Performing reflection for task: %s", question.TaskID)

	historyBytes, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal communication history: %w", err)
	}

	prompt := fmt.Sprintf(ReflectionPrompt,
		question.Question,
		sopContent,
		string(historyBytes),
		question.FinalAnswer,
		question.AnnotatorMetadata.Steps,
	)

	reflectorAgent, err := agent.NewBaseAgent(
		agent.WithName("ReflectorAgent"),
		agent.WithDesc("An agent that reflects on the failure of a multi-agent system and suggests improvements."),
		agent.WithPrompt(prompt),
		agent.WithLLM(client),
		agent.WithInstruction(""),
		agent.WithSuffix(""), // Use a null suffix
	)
	if err != nil {
		return fmt.Errorf("failed to create ReflectorAgent: %w", err)
	}

	log.Println("Calling LLM for reflection...")
	gen, err := reflectorAgent.Run(context.Background(), []schema.Message{
		{
			Type:    schema.MsgTypeMsg,
			Content: "You are an expert in analyzing and refining multi-agent systems.",
		},
	}, llm.WithTemperature(0.6))
	if err != nil {
		return fmt.Errorf("ReflectorAgent run failed: %w", err)
	}

	if len(gen.Messages) == 0 || gen.Messages[0].Content == "" {
		return fmt.Errorf("LLM returned an empty response for reflection")
	}

	agentResponse := gen.Messages[0]
	log.Printf("Reflector Agent Thought: %s", agentResponse.Thought)

	// The agent's response content is expected to be a JSON string.
	// We need to unmarshal it to pretty-print it.
	var reflectionOutput interface{}
	if err := json.Unmarshal([]byte(agentResponse.Content), &reflectionOutput); err != nil {
		return fmt.Errorf("failed to unmarshal reflection JSON from agent response, content was:\n%s\nError: %w", agentResponse.Content, err)
	}

	// Pretty print the full JSON structure for saving
	prettyJSON, err := json.MarshalIndent(reflectionOutput, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pretty reflection JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, prettyJSON, 0644); err != nil {
		return fmt.Errorf("failed to write reflection to file %s: %w", outputPath, err)
	}

	log.Printf("Successfully wrote reflection to %s", outputPath)
	return nil
}

func main() {
	// 大模型实例化
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	evalLogPath := "../../eval/eval_level_0_sopv1_20250722155317.json"
	trainDataPath := "../../../../../dataset/gaia/train.json"
	sopDir := "../"
	reflectionOutDir := "./"

	var results []ResultLog
	if err := loadFile(evalLogPath, &results); err != nil {
		log.Fatalf("Error loading eval log: %v", err)
	}

	var questions []GaiaQuestion
	if err := loadFile(trainDataPath, &questions); err != nil {
		log.Fatalf("Error loading train data: %v", err)
	}

	questionsByTaskID := make(map[string]GaiaQuestion)
	for _, q := range questions {
		questionsByTaskID[q.TaskID] = q
	}

	for _, result := range results {
		if !result.IsCorrect {
			log.Printf("Found failed task ID: %s (Question Index: %d)", result.TaskID, result.ID)

			question, ok := questionsByTaskID[result.TaskID]
			if !ok {
				log.Printf("Warning: Could not find question data for task ID %s. Skipping.", result.TaskID)
				continue
			}

			sopPath := filepath.Join(sopDir, fmt.Sprintf("gen_sop_v1_L0_q%d.json", result.ID))
			sopBytes, err := os.ReadFile(sopPath)
			if err != nil {
				log.Printf("Warning: Could not read SOP file %s for failed task. Skipping. Error: %v", sopPath, err)
				continue
			}

			reflectionOutputPath := filepath.Join(reflectionOutDir, fmt.Sprintf("ref_v1_L0_q%d.json", result.ID))

			if err := performReflection(client, string(sopBytes), result.CommunicationHistory, question, reflectionOutputPath); err != nil {
				log.Printf("ERROR: Failed to perform reflection for task %s: %v", result.TaskID, err)
			}
		}
	}

	log.Println("Reflection process finished.")
}
