package main

import (
	"bufio"
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

// HumanEvalQuestion represents a single question from the HumanEval dataset
type HumanEvalQuestion struct {
	TaskID            string `json:"task_id"`
	Prompt            string `json:"prompt"`
	CanonicalSolution string `json:"canonical_solution"`
	Test              string `json:"test"`
	EntryPoint        string `json:"entry_point"`
}

// HumanEvalResultLog represents the execution result for a single question
type HumanEvalResultLog struct {
	ID                   int              `json:"id"`
	Query                string           `json:"query"`
	ModelOutput          string           `json:"model_output"`
	CommunicationHistory []schema.Message `json:"communication_history"`
	TotalCount           int              `json:"total_count"`
	Time                 string           `json:"time"`
}

// HumanEvalEvaluationResult represents the evaluation result with test outcomes
type HumanEvalEvaluationResult struct {
	ID          int    `json:"id"`
	TaskID      string `json:"task_id"`
	TestResults string `json:"test_results"`
	Success     bool   `json:"success"`
}

// Struct for SOP file
type SOPFile struct {
	Question string `json:"question"`
	Analysis string `json:"analysis"`
	SOPs     []SOP  `json:"sops"`
}

type SOP struct {
	Team     []string      `json:"team"`
	Workflow string        `json:"workflow"`
	Details  []AgentDetail `json:"details"`
}

type AgentDetail struct {
	Name           string   `json:"name"`
	Responsibility string   `json:"responsibility"`
	Instruction    string   `json:"instruction"`
	Tools          []string `json:"tools"`
}

// ReflectionOutput defines the structure for the reflection JSON file.
type ReflectionOutput struct {
	Question      string          `json:"question"`
	OriginalSOP   string          `json:"workflow"`
	HistoryString string          `json:"history_conversation"`
	LLMReflection json.RawMessage `json:"llm_reflection"`
	GroundTruth   string          `json:"ground_truth"`
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

// loadHumanEvalDataset loads the HumanEval dataset from a JSONL file
func loadHumanEvalDataset(filePath string) (map[int]HumanEvalQuestion, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	questions := make(map[int]HumanEvalQuestion)
	scanner := bufio.NewScanner(file)
	i := 0
	for scanner.Scan() {
		var question HumanEvalQuestion
		if err := json.Unmarshal(scanner.Bytes(), &question); err != nil {
			return nil, err
		}
		questions[i] = question
		i++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return questions, nil
}

// loadEvaluationResults reads the JSONL evaluation results file
func loadEvaluationResults(filePath string) (map[int]HumanEvalEvaluationResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open evaluation file %s: %w", filePath, err)
	}
	defer file.Close()

	results := make(map[int]HumanEvalEvaluationResult)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var evalResult HumanEvalEvaluationResult
		if err := json.Unmarshal([]byte(line), &evalResult); err != nil {
			log.Printf("Warning: failed to unmarshal evaluation result line: %v", err)
			continue
		}

		results[evalResult.ID] = evalResult
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading evaluation file: %w", err)
	}

	return results, nil
}

// filterCommunicationHistory filters the communication history to only include Sender, Receiver, and Content
func filterCommunicationHistory(messages []schema.Message) []map[string]string {
	filtered := make([]map[string]string, len(messages))
	for i, msg := range messages {
		filtered[i] = map[string]string{
			"sender":   msg.Sender,
			"receiver": msg.Receiver,
			"content":  msg.Content,
		}
	}
	return filtered
}

func performReflection(client llm.LLM, sopContent string, historyString string, question HumanEvalQuestion, evalResult HumanEvalEvaluationResult, modelOutput string, outputPath string) error {
	log.Printf("Performing reflection for task ID: %s", question.TaskID)

	constraintInfo := map[string]interface{}{
		"test_results": evalResult.TestResults,
		"success":      evalResult.Success,
	}
	constraintBytes, err := json.MarshalIndent(constraintInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal constraint information: %w", err)
	}
	constraintString := string(constraintBytes)

	prompt := fmt.Sprintf(ReflectionPrompt,
		question.Prompt,
		modelOutput,
		constraintString,
		sopContent,
		historyString,
	)

	reflectorAgent, err := agent.NewBaseAgent(
		agent.WithName("ReflectorAgent"),
		agent.WithDesc("An agent that reflects on the failure of a multi-agent system for code generation and suggests improvements."),
		agent.WithPrompt(prompt),
		agent.WithLLM(client),
		agent.WithInstruction(""),
		agent.WithSuffix(""),
	)
	if err != nil {
		return fmt.Errorf("failed to create ReflectorAgent: %w", err)
	}

	log.Println("Calling LLM for reflection...")
	gen, err := reflectorAgent.Run(context.Background(), []schema.Message{
		{
			Type:    schema.MsgTypeMsg,
			Content: "You are an expert in analyzing and refining multi-agent systems for code generation.",
		},
	}, llm.WithTemperature(0.6), llm.WithTopP(0.95))
	if err != nil {
		return fmt.Errorf("ReflectorAgent run failed: %w", err)
	}

	if len(gen.Messages) == 0 || gen.Messages[0].Content == "" {
		return fmt.Errorf("LLM returned an empty response for reflection")
	}

	agentResponse := gen.Messages[0]
	log.Printf("Reflector Agent Thought: %s", agentResponse.Thought)

	var llmReflection json.RawMessage
	if err := json.Unmarshal([]byte(agentResponse.Content), &llmReflection); err != nil {
		log.Printf("LLM reflection response is not valid JSON, wrapping it. Error: %v", err)
		escapedString, _ := json.Marshal(agentResponse.Content)
		llmReflection = json.RawMessage(escapedString)
	}

	var sopFile SOPFile
	if err := json.Unmarshal([]byte(sopContent), &sopFile); err != nil {
		log.Printf("Warning: could not unmarshal original SOP content: %v", err)
		sopFile.SOPs = []SOP{}
	}

	outputData := ReflectionOutput{
		Question:      question.Prompt,
		OriginalSOP:   sopFile.SOPs[0].Workflow,
		HistoryString: historyString,
		LLMReflection: llmReflection,
		GroundTruth:   question.CanonicalSolution,
	}

	prettyJSON, err := json.MarshalIndent(outputData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal combined reflection data: %w", err)
	}

	if err := os.WriteFile(outputPath, prettyJSON, 0644); err != nil {
		return fmt.Errorf("failed to write reflection to file %s: %w", outputPath, err)
	}

	log.Printf("Successfully wrote reflection to %s -------------\n", outputPath)
	return nil
}

func performRevision(client llm.LLM, originalSopBytes []byte, reflectionBytes []byte, outputPath string) error {
	log.Printf("Performing revision for SOP: %s", outputPath)

	template_path := "humaneval_sop.json"
	templateBytes, _ := os.ReadFile(template_path)
	var sopTemplate SOP
	if err := json.Unmarshal(templateBytes, &sopTemplate); err != nil {
		return fmt.Errorf("failed to unmarshal SOP template: %w", err)
	}
	templateBytes, err := json.MarshalIndent(sopTemplate, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SOP template to string: %w", err)
	}
	templateString := string(templateBytes)

	var reflectionInput ReflectionOutput
	if err := json.Unmarshal(reflectionBytes, &reflectionInput); err != nil {
		return fmt.Errorf("failed to unmarshal reflection file content: %w", err)
	}

	reflectionContent := string(reflectionInput.LLMReflection)
	if len(reflectionContent) == 0 || reflectionContent == "null" {
		return fmt.Errorf("LLMReflection field is missing or empty in the reflection file")
	}

	originalSopContent := string(originalSopBytes)

	prompt := fmt.Sprintf(RevisionPrompt,
		templateString,
		originalSopContent,
		reflectionContent,
	)

	reviserAgent, err := agent.NewBaseAgent(
		agent.WithName("ReviserAgent"),
		agent.WithDesc("An agent that revises a Standard Operating Procedure based on reflection of a past failure."),
		agent.WithPrompt(prompt),
		agent.WithLLM(client),
		agent.WithInstruction(""),
		agent.WithSuffix(""),
	)
	if err != nil {
		return fmt.Errorf("failed to create ReviserAgent: %w", err)
	}

	log.Println("Calling LLM for revision...")
	gen, err := reviserAgent.Run(context.Background(), []schema.Message{
		{
			Type:    schema.MsgTypeMsg,
			Content: "You are an expert multi-agent system designer for code generation.",
		},
	}, llm.WithTemperature(0.6), llm.WithTopP(0.95))
	if err != nil {
		return fmt.Errorf("ReviserAgent run failed: %w", err)
	}

	if len(gen.Messages) == 0 || gen.Messages[0].Content == "" {
		return fmt.Errorf("LLM returned an empty response for revision")
	}

	revisedSopJSON := gen.Messages[0].Content
	var sopFile SOPFile
	if err := json.Unmarshal([]byte(revisedSopJSON), &sopFile); err != nil {
		return fmt.Errorf("failed to parse LLM response as SOPFile. Content was:\n%s", revisedSopJSON)
	}

	prettyJSON, err := json.MarshalIndent(sopFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pretty revised SOP JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, prettyJSON, 0644); err != nil {
		return fmt.Errorf("failed to write revised SOP to file %s: %w", outputPath, err)
	}

	log.Printf("Successfully wrote revised SOP to %s ----------\n\n", outputPath)
	return nil
}

func main() {
	// --- CONFIGURATION ---
	evalLogPath := "../output/humaneval_results_20250926120000.json" // Placeholder
	datasetPath := "../../../dataset/humaneval/test.jsonl"
	evaluationResultsPath := "../results/humaneval_eval_results.jsonl" // Placeholder
	sopDir := "./"
	reflectionOutDir := "./reflect/"
	revisionOutDir := "./rev_sop/"
	// --- END CONFIGURATION ---

	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	if err := os.MkdirAll(reflectionOutDir, 0755); err != nil {
		log.Fatalf("Failed to create reflection directory: %v", err)
	}
	if err := os.MkdirAll(revisionOutDir, 0755); err != nil {
		log.Fatalf("Failed to create revision directory: %v", err)
	}

	var results []HumanEvalResultLog
	if err := loadFile(evalLogPath, &results); err != nil {
		log.Fatalf("Error loading eval log: %v", err)
	}

	questions, err := loadHumanEvalDataset(datasetPath)
	if err != nil {
		log.Fatalf("Error loading humaneval dataset: %v", err)
	}

	evaluationResults, err := loadEvaluationResults(evaluationResultsPath)
	if err != nil {
		log.Fatalf("Error loading evaluation results: %v", err)
	}

	historyStrings := make(map[int]string)
	for _, result := range results {
		filteredHistory := filterCommunicationHistory(result.CommunicationHistory)
		historyBytes, err := json.MarshalIndent(filteredHistory, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal communication history for query %d: %v", result.ID, err)
		}
		historyStrings[result.ID] = string(historyBytes)
	}

	for i, result := range results {
		fmt.Printf("\n==================Processing result ID: %d\n", result.ID)

		sopPath := filepath.Join(sopDir, "humaneval_sop.json")
		reflectionOutputPath := filepath.Join(reflectionOutDir, fmt.Sprintf("ref_humaneval_q%d.json", result.ID))
		revisedSopPath := filepath.Join(revisionOutDir, fmt.Sprintf("rev_humaneval_q%d.json", result.ID))

		sopBytes, err := os.ReadFile(sopPath)
		if err != nil {
			log.Printf("Warning: Could not read original SOP file %s. Skipping. Error: %v", sopPath, err)
			continue
		}

		question, ok := questions[result.ID]
		if !ok {
			log.Printf("Warning: Could not find question data for ID %d. Skipping.", result.ID)
			continue
		}

		evalResult, ok := evaluationResults[result.ID]
		if !ok {
			log.Printf("Warning: Could not find evaluation result for ID %d. Skipping.", result.ID)
			continue
		}

		if evalResult.Success {
			log.Printf("Query ID %d was successful. Skipping reflection and revision.", result.ID)
			continue
		}

		historyString, ok := historyStrings[result.ID]
		if !ok {
			log.Printf("Warning: Could not find history for ID %d. Skipping.", result.ID)
			continue
		}

		if err := performReflection(client, string(sopBytes), historyString, question, evalResult, result.ModelOutput, reflectionOutputPath); err != nil {
			log.Printf("ERROR: Failed to perform reflection for query %d: %v", i, err)
			continue
		}

		reflectionBytes, err := os.ReadFile(reflectionOutputPath)
		if err != nil {
			log.Printf("ERROR: Failed to read reflection file %s for revision. Skipping. Error: %v", reflectionOutputPath, err)
			continue
		}

		if err := performRevision(client, sopBytes, reflectionBytes, revisedSopPath); err != nil {
			log.Printf("ERROR: Failed to perform revision for query %d: %v", i, err)
		}
	}

	log.Println("Revision process finished for humaneval.")
}
