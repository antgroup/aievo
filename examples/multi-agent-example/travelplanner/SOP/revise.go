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

// TravelPlannerQuestion represents a single question from the TravelPlanner dataset
type TravelPlannerQuestion struct {
	Org                  string `json:"org"`
	Dest                 string `json:"dest"`
	Days                 int    `json:"days"`
	VisitingCityNumber   int    `json:"visiting_city_number"`
	Date                 string `json:"date"`
	PeopleNumber         int    `json:"people_number"`
	LocalConstraint      string `json:"local_constraint"`
	Budget               int    `json:"budget"`
	Query                string `json:"query"`
	Level                string `json:"level"`
	ReferenceInformation string `json:"reference_information"`
	AnnotatedPlan        string `json:"annotated_plan"`
}

// TravelPlannerResultLog represents the evaluation result for a single question
type TravelPlannerResultLog struct {
	ID                   int              `json:"id"`
	Query                string           `json:"query"`
	ModelOutput          string           `json:"model_output"`
	CommunicationHistory []schema.Message `json:"communication_history"`
	TotalCount           int              `json:"total_count"`
	Time                 string           `json:"time"`
}

// TravelPlannerEvaluationResult represents the evaluation result with constraints
type TravelPlannerEvaluationResult struct {
	Idx                   int                    `json:"idx"`
	Query                 string                 `json:"query"`
	Plan                  []json.RawMessage      `json:"plan"`
	CommonsenseConstraint map[string]interface{} `json:"commonsense_constraint"`
	HardConstraint        interface{}            `json:"hard_constraint"`
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

// ReflectionOutput defines the structure for the reflection JSON file.
type ReflectionOutput struct {
	Question      string          `json:"question"`
	OriginalSOP   string          `json:"sop"`
	HistoryString string          `json:"history_conversation"`
	LLMReflection json.RawMessage `json:"llm_reflection"`
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

// loadEvaluationResults reads the JSONL evaluation results file
func loadEvaluationResults(filePath string) (map[int]TravelPlannerEvaluationResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open evaluation file %s: %w", filePath, err)
	}
	defer file.Close()

	results := make(map[int]TravelPlannerEvaluationResult)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var evalResult TravelPlannerEvaluationResult
		if err := json.Unmarshal([]byte(line), &evalResult); err != nil {
			log.Printf("Warning: failed to unmarshal evaluation result line: %v", err)
			continue
		}

		results[evalResult.Idx] = evalResult
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

func performReflection(client llm.LLM, sopContent string, historyString string, question TravelPlannerQuestion, evalResult TravelPlannerEvaluationResult, outputPath string) error {
	log.Printf("Performing reflection for query: %s", question.Query)

	// Format the system generated plan from evaluation result
	planBytes, err := json.MarshalIndent(evalResult.Plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal system generated plan: %w", err)
	}
	systemGeneratedPlan := string(planBytes)

	// Format the evaluation result into a readable string
	evalResultBytes, err := json.MarshalIndent(evalResult, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal evaluation result: %w", err)
	}
	evalResultString := string(evalResultBytes)

	prompt := fmt.Sprintf(ReflectionPrompt,
		question.Query,
		question.AnnotatedPlan,
		systemGeneratedPlan,
		evalResultString,
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

	// Create the structured output
	// Unmarshal the agent's response to ensure it's valid JSON
	var llmReflection json.RawMessage
	if err := json.Unmarshal([]byte(agentResponse.Content), &llmReflection); err != nil {
		// Fallback for non-JSON response
		log.Printf("LLM reflection response is not valid JSON, wrapping it. Error: %v", err)
		escapedString, _ := json.Marshal(agentResponse.Content)
		llmReflection = json.RawMessage(escapedString)
	}

	// Unmarshal original SOP to a structured format to ensure it's well-formed in the final JSON
	var sopFile SOPFile
	if err := json.Unmarshal([]byte(sopContent), &sopFile); err != nil {
		log.Printf("Warning: could not unmarshal original SOP content: %v", err)
		// If unmarshalling fails, just use the raw string.
		sopFile.SOPs = []SOP{} // or handle error appropriately
	}

	outputData := ReflectionOutput{
		Question:      question.Query,
		OriginalSOP:   sopFile.SOPs[0].SOP, // Assuming the first SOP is the main one,
		HistoryString: historyString,
		LLMReflection: llmReflection,
	}

	// Marshal the combined data with pretty printing
	prettyJSON, err := json.MarshalIndent(outputData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal combined reflection data: %w", err)
	}

	// Write the final JSON to the output file
	if err := os.WriteFile(outputPath, prettyJSON, 0644); err != nil {
		return fmt.Errorf("failed to write reflection to file %s: %w", outputPath, err)
	}

	log.Printf("Successfully wrote reflection to %s -------------\n", outputPath)
	return nil
}

func performRevision(client llm.LLM, originalSopBytes []byte, reflectionBytes []byte, outputPath string) error {
	log.Printf("Performing revision for SOP: %s", outputPath)

	// 1. Unmarshal the reflection file to get the LLM's reflection part.
	var reflectionInput ReflectionOutput
	if err := json.Unmarshal(reflectionBytes, &reflectionInput); err != nil {
		return fmt.Errorf("failed to unmarshal reflection file content: %w", err)
	}

	// 2. Extract only the LLM's reflection for the prompt.
	reflectionContent := string(reflectionInput.LLMReflection)
	if len(reflectionContent) == 0 || reflectionContent == "null" {
		return fmt.Errorf("LLMReflection field is missing or empty in the reflection file")
	}

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

	log.Printf("Successfully wrote revised SOP to %s ----------\n\n", outputPath)
	return nil
}

func main() {
	// --- CONFIGURATION ---

	evalLogPath := "../output/train_t1.1_20250822104827.json"
	trainDataPath := "../../../../dataset/travelplanner/train/travelplanner_train_split.json"
	evaluationResultsPath := "../results/train_v0_20250819203412_per_results_20250822_112057.jsonl"
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

	var results []TravelPlannerResultLog
	if err := loadFile(evalLogPath, &results); err != nil {
		log.Fatalf("Error loading eval log: %v", err)
	}

	var questions []TravelPlannerQuestion
	if err := loadFile(trainDataPath, &questions); err != nil {
		log.Fatalf("Error loading train data: %v", err)
	}

	// Load evaluation results
	evaluationResults, err := loadEvaluationResults(evaluationResultsPath)
	if err != nil {
		log.Fatalf("Error loading evaluation results: %v", err)
	}

	questionsByID := make(map[int]TravelPlannerQuestion)
	for i, q := range questions {
		questionsByID[i] = q
	}

	// --- History Loading Logic ---
	historyStrings := make([]string, len(results))
	log.Println("Loading communication history from eval log file.")
	for i, result := range results {
		filteredHistory := filterCommunicationHistory(result.CommunicationHistory)
		historyBytes, err := json.MarshalIndent(filteredHistory, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal communication history for query %d: %v", result.ID, err)
		}
		historyStrings[i] = string(historyBytes)
	}

	// --- End History Loading Logic ---

	for i, result := range results {
		fmt.Printf("\n==================Processing question ID: %d\n", i)

		sopPath := filepath.Join(sopDir, fmt.Sprintf("gen_sop_v1_q%d.json", result.ID))
		revisedSopPath := filepath.Join(revisionOutDir, fmt.Sprintf("rev_sop_v1.1_q%d.json", result.ID))

		sopBytes, err := os.ReadFile(sopPath)
		if err != nil {
			log.Printf("Warning: Could not read original SOP file %s. Skipping. Error: %v", sopPath, err)
			continue
		}

		// For TravelPlanner, all tasks need reflection (no IsCorrect field)
		log.Printf("Processing travel planner query ID: %d", result.ID)

		question, ok := questionsByID[result.ID]
		if !ok {
			log.Printf("Warning: Could not find question data for ID %d. Skipping.", result.ID)
			continue
		}

		// Get evaluation result for this query
		evalResult, ok := evaluationResults[result.ID]
		if !ok {
			log.Printf("Warning: Could not find evaluation result for ID %d. Skipping.", result.ID)
			continue
		}

		reflectionOutputPath := filepath.Join(reflectionOutDir, fmt.Sprintf("ref_v1.1_q%d.json", result.ID))

		// Use the pre-processed history string
		historyString := historyStrings[i]

		if err := performReflection(client, string(sopBytes), historyString, question, evalResult, reflectionOutputPath); err != nil {
			log.Printf("ERROR: Failed to perform reflection for query %d: %v", result.ID, err)
			continue // Skip revision if reflection fails
		}

		// Now, read the reflection file and perform revision
		reflectionBytes, err := os.ReadFile(reflectionOutputPath)
		if err != nil {
			log.Printf("ERROR: Failed to read reflection file %s for revision. Skipping. Error: %v", reflectionOutputPath, err)
			continue
		}

		if err := performRevision(client, sopBytes, reflectionBytes, revisedSopPath); err != nil {
			log.Printf("ERROR: Failed to perform revision for query %d: %v", result.ID, err)
		}
	}

	log.Println("Revision process finished.-----------------------------------")
}
