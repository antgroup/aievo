package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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

// ReflectionInput holds the data needed for the reflection prompt.
type ReflectionInput struct {
	Question             string `json:"question"`
	SOP                  string `json:"sop"`
	CommunicationHistory string `json:"communication_history"`
	FinalAnswer          string `json:"final_answer"`
	ExpertAnalysis       string `json:"expert_analysis"`
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

func performReflection(client llm.LLM, sopContent string, historyString string, question GaiaQuestion, outputPath string) error {
	log.Printf("Performing reflection for task: %s", question.TaskID)

	prompt := fmt.Sprintf(ReflectionPrompt,
		question.Question,
		question.FinalAnswer,
		question.AnnotatorMetadata.Steps,
		sopContent,
		historyString,
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

	log.Printf("Successfully wrote reflection to %s -------------", outputPath)
	return nil
}

func performRevision(client llm.LLM, originalSopBytes []byte, reflectionBytes []byte, outputPath string) error {
	log.Printf("Performing revision for SOP: %s", outputPath)

	// The reflection content is already a JSON string.
	reflectionContent := string(reflectionBytes)
	originalSopContent := string(originalSopBytes)

	prompt := fmt.Sprintf(RevisionPrompt,
		originalSopContent,
		reflectionContent,
	)

	reviserAgent, err := agent.NewBaseAgent(
		agent.WithName("ReviserAgent"),
		agent.WithDesc("An agent that revises a Standard Operating Procedure based on reflection of a past failure."),
		agent.WithPrompt(prompt),
		agent.WithLLM(client),
		agent.WithInstruction(""),
		agent.WithSuffix(""), // Use a null suffix
	)
	if err != nil {
		return fmt.Errorf("failed to create ReviserAgent: %w", err)
	}

	log.Println("Calling LLM for revision...")
	gen, err := reviserAgent.Run(context.Background(), []schema.Message{
		{
			Type:    schema.MsgTypeMsg,
			Content: "You are an expert multi-agent system designer.",
		},
	}, llm.WithTemperature(0.6))
	if err != nil {
		return fmt.Errorf("ReviserAgent run failed: %w", err)
	}

	if len(gen.Messages) == 0 || gen.Messages[0].Content == "" {
		return fmt.Errorf("LLM returned an empty response for revision")
	}

	revisedSopJSON := gen.Messages[0].Content

	// The output should be a SOPFile structure, let's validate and format it.
	var sopFile SOPFile
	if err := json.Unmarshal([]byte(revisedSopJSON), &sopFile); err != nil {
		// Fallback: maybe it just returned the SOP array
		var sops []SOP
		if err2 := json.Unmarshal([]byte(revisedSopJSON), &sops); err2 != nil {
			return fmt.Errorf("failed to unmarshal revised SOP JSON from agent response, content was:\n%s\nError: %w", revisedSopJSON, err)
		}
		// If fallback is successful, wrap it in SOPFile
		var originalSopFile SOPFile
		_ = json.Unmarshal(originalSopBytes, &originalSopFile)
		sopFile.Question = originalSopFile.Question
		sopFile.Analysis = gen.Messages[0].Thought
		sopFile.SOPs = sops
	}

	// Pretty print the full JSON structure for saving
	prettyJSON, err := json.MarshalIndent(sopFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pretty revised SOP JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, prettyJSON, 0644); err != nil {
		return fmt.Errorf("failed to write revised SOP to file %s: %w", outputPath, err)
	}

	log.Printf("Successfully wrote revised SOP to %s ----------", outputPath)
	return nil
}

func main() {
	// --- CONFIGURATION ---
	// historySourceFlag: 0 for reading from eval log, 1 for reading from raw log file
	historySourceFlag := 1
	evalLogPath := "../eval/eval_level_0_v3_20250805102200.json"
	rawHistoryLogPath := "../eval/log_output_v3_2025-0805.log" // New log file path
	trainDataPath := "../../../../dataset/gaia/train.json"
	sopDir := "./gen_sop/"
	reflectionOutDir := "./reflect/"
	revisionOutDir := "./rev_sop/"
	// --- END CONFIGURATION ---

	// 大模型实例化
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	// Ensure output directories exist
	if err := os.MkdirAll(reflectionOutDir, 0755); err != nil {
		log.Fatalf("Failed to create reflection directory: %v", err)
	}
	if err := os.MkdirAll(revisionOutDir, 0755); err != nil {
		log.Fatalf("Failed to create revision directory: %v", err)
	}

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

	// --- History Loading Logic ---
	historyStrings := make([]string, len(results))
	if historySourceFlag == 1 {
		log.Printf("Loading communication history from raw log file: %s", rawHistoryLogPath)
		logBytes, err := os.ReadFile(rawHistoryLogPath)
		if err != nil {
			log.Fatalf("Failed to read raw history log file: %v", err)
		}
		logContent := string(logBytes)
		// Split by the specified delimiter
		splitHistories := strings.Split(logContent, "History: (User")
		if len(splitHistories) < 2 {
			log.Fatalf("Raw log file does not contain the expected delimiter 'History: (User'")
		}
		// The first element before the first split is usually empty or irrelevant, so we skip it.
		rawHistories := splitHistories[1:]
		if len(rawHistories) != len(results) {
			log.Fatalf("Mismatch in history count: expected %d, but got %d from raw log file", len(results), len(rawHistories))
		}
		for i, h := range rawHistories {
			// Re-add the delimiter that was removed by splitting
			historyStrings[i] = "History: (User" + h
		}
		log.Println("Successfully loaded and parsed history from raw log file.")
	} else {
		log.Println("Loading communication history from eval log file.")
		for i, result := range results {
			historyBytes, err := json.MarshalIndent(result.CommunicationHistory, "", "  ")
			if err != nil {
				log.Fatalf("Failed to marshal communication history for task %s: %v", result.TaskID, err)
			}
			historyStrings[i] = string(historyBytes)
		}
	}
	// --- End History Loading Logic ---

	for i, result := range results {
		sopPath := filepath.Join(sopDir, fmt.Sprintf("gen_sop_v3_L0_q%d.json", result.ID))
		revisedSopPath := filepath.Join(revisionOutDir, fmt.Sprintf("rev_sop_v3.1_L0_q%d.json", result.ID))

		sopBytes, err := os.ReadFile(sopPath)
		if err != nil {
			log.Printf("Warning: Could not read original SOP file %s. Skipping. Error: %v", sopPath, err)
			continue
		}

		if result.IsCorrect {
			log.Printf("Task %s was successful. Copying original SOP to %s", result.TaskID, revisedSopPath)
			if err := os.WriteFile(revisedSopPath, sopBytes, 0644); err != nil {
				log.Printf("ERROR: Failed to copy successful SOP for task %s: %v", result.TaskID, err)
			}
			continue
		}

		// If the task failed, perform reflection and revision
		log.Printf("Found failed task ID: %s (Question Index: %d)", result.TaskID, result.ID)

		question, ok := questionsByTaskID[result.TaskID]
		if !ok {
			log.Printf("Warning: Could not find question data for task ID %s. Skipping.", result.TaskID)
			continue
		}

		reflectionOutputPath := filepath.Join(reflectionOutDir, fmt.Sprintf("ref_v3.1_L0_q%d.json", result.ID))

		// Use the pre-processed history string
		historyString := historyStrings[i]

		if err := performReflection(client, string(sopBytes), historyString, question, reflectionOutputPath); err != nil {
			log.Printf("ERROR: Failed to perform reflection for task %s: %v", result.TaskID, err)
			continue // Skip revision if reflection fails
		}

		// Now, read the reflection file and perform revision
		reflectionBytes, err := os.ReadFile(reflectionOutputPath)
		if err != nil {
			log.Printf("ERROR: Failed to read reflection file %s for revision. Skipping. Error: %v", reflectionOutputPath, err)
			continue
		}

		if err := performRevision(client, sopBytes, reflectionBytes, revisedSopPath); err != nil {
			log.Printf("ERROR: Failed to perform revision for task %s: %v", result.TaskID, err)
		}
	}

	log.Println("Revision process finished.-----------------------------------")
}
