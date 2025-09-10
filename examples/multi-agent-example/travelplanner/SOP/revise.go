package main

import (
	"bufio"
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
	Success               bool                   `json:"success"`
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

// extractPurePlan extracts the pure plan (last array) from annotated_plan string
func extractPurePlan(annotatedPlan string) string {
	if annotatedPlan == "" {
		return ""
	}

	// Find the first occurrence of [{'days': 1,
	pattern := "[{'days': 1,"
	index := strings.Index(annotatedPlan, pattern)

	if index == -1 {
		// If the pattern is not found, try alternative patterns
		altPatterns := []string{
			"[{\"days\": 1,", // With double quotes
			"[{'days':1,",    // Without space after colon
			"[{\"days\":1,",  // Double quotes without space
		}

		for _, altPattern := range altPatterns {
			index = strings.Index(annotatedPlan, altPattern)
			if index != -1 {
				// pattern = altPattern
				break
			}
		}
	}

	if index == -1 {
		log.Printf("Warning: could not find pure plan pattern in annotated_plan: %s", annotatedPlan)
		return annotatedPlan // Return original if pattern not found
	}

	// Extract everything from the pattern onwards
	purePlan := annotatedPlan[index:]

	// Find the matching closing bracket to get complete array
	// We need to find the last ]] to close the array properly
	lastIndex := strings.LastIndex(purePlan, "]]")
	if lastIndex != -1 {
		purePlan = purePlan[:lastIndex+2] // +2 to include the ]]
	}

	log.Printf("Successfully extracted pure plan starting from position %d", index)
	return purePlan
}

// generateSuccessfulReflection generates a reflection file for successful cases without LLM reflection
func generateSuccessfulReflection(sopContent string, historyString string, question TravelPlannerQuestion, outputPath string) error {
	log.Printf("Generating reflection for successful query: %s", question.Query)

	// Unmarshal original SOP to a structured format to ensure it's well-formed in the final JSON
	var sopFile SOPFile
	if err := json.Unmarshal([]byte(sopContent), &sopFile); err != nil {
		log.Printf("Warning: could not unmarshal original SOP content: %v", err)
		// If unmarshalling fails, just use the raw string.
		sopFile.SOPs = []SOP{} // or handle error appropriately
	}

	// Extract pure plan from annotated_plan
	purePlan := extractPurePlan(question.AnnotatedPlan)

	// For successful cases, we don't have LLM reflection, so we use an empty JSON object
	llmReflection := json.RawMessage("{}")

	outputData := ReflectionOutput{
		Question:      question.Query,
		OriginalSOP:   sopFile.SOPs[0].Workflow, // Assuming the first SOP is the main one,
		HistoryString: historyString,
		LLMReflection: llmReflection,
		GroundTruth:   purePlan,
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

	log.Printf("Successfully wrote successful case reflection to %s -------------\n", outputPath)
	return nil
}

func performReflection(client llm.LLM, sopContent string, historyString string, question TravelPlannerQuestion, evalResult TravelPlannerEvaluationResult, outputPath string) error {
	log.Printf("Performing reflection for query: %s", question.Query)

	// Format the system generated plan from evaluation result
	planBytes, err := json.MarshalIndent(evalResult.Plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal system generated plan: %w", err)
	}
	systemGeneratedPlan := string(planBytes)

	// Format only the constraint fields from evaluation result
	constraintInfo := map[string]interface{}{
		"commonsense_constraint": evalResult.CommonsenseConstraint,
		"hard_constraint":        evalResult.HardConstraint,
	}
	constraintBytes, err := json.MarshalIndent(constraintInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal constraint information: %w", err)
	}
	constraintString := string(constraintBytes)

	prompt := fmt.Sprintf(ReflectionPrompt,
		question.Query,
		systemGeneratedPlan,
		constraintString,
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
	}, llm.WithTemperature(0.6), llm.WithTopP(0.95))
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

	// Extract pure plan from annotated_plan
	purePlan := extractPurePlan(question.AnnotatedPlan)

	outputData := ReflectionOutput{
		Question:      question.Query,
		OriginalSOP:   sopFile.SOPs[0].Workflow, // Assuming the first SOP is the main one,
		HistoryString: historyString,
		LLMReflection: llmReflection,
		GroundTruth:   purePlan,
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

	template_path := "v3.json"
	templateBytes, _ := os.ReadFile(template_path)
	var sops []SOP
	if err := json.Unmarshal(templateBytes, &sops); err != nil {
		return fmt.Errorf("failed to unmarshal SOP JSON in either new or old format: %w", err)
	}
	templateSOP := sops[len(sops)-1] // Get the last one as template
	templateBytes, err := json.MarshalIndent(templateSOP, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SOP template to string: %w", err)
	}
	templateString := string(templateBytes)

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
	}, llm.WithTemperature(0.6), llm.WithTopP(0.95))
	if err != nil {
		return fmt.Errorf("ReviserAgent run failed: %w", err)
	}

	if len(gen.Messages) == 0 || gen.Messages[0].Content == "" {
		return fmt.Errorf("LLM returned an empty response for revision")
	}

	revisedSopJSON := gen.Messages[0].Content
	log.Printf("LLM returned content length: %d", len(revisedSopJSON))

	// 先解析原始SOP文件以获取question信息
	var originalSopFile SOPFile
	originalQuestion := "Travel Planning Question"
	if err := json.Unmarshal(originalSopBytes, &originalSopFile); err == nil && originalSopFile.Question != "" {
		originalQuestion = originalSopFile.Question
	}

	// 尝试多种解析方式
	var sopFile SOPFile
	var parseSuccess bool

	// 方式1: 直接解析Content为SOP对象或SOP数组
	// 尝试解析为单个SOP对象
	var sop SOP
	if err := json.Unmarshal([]byte(revisedSopJSON), &sop); err == nil && len(sop.Team) > 0 {
		sopFile = SOPFile{
			Question: originalQuestion,
			Analysis: gen.Messages[0].Thought,
			SOPs:     []SOP{sop},
		}
		parseSuccess = true
		log.Printf("Successfully parsed Content as single SOP")
	} else {
		// 尝试解析为SOP数组
		var sops []SOP
		if err := json.Unmarshal([]byte(revisedSopJSON), &sops); err == nil && len(sops) > 0 {
			sopFile = SOPFile{
				Question: originalQuestion,
				Analysis: gen.Messages[0].Thought,
				SOPs:     sops,
			}
			parseSuccess = true
			log.Printf("Successfully parsed Content as SOP array with %d SOPs", len(sops))
		}
	}

	// 方式2: 如果方式1失败，尝试解析为完整的SOPFile
	if !parseSuccess {
		if err := json.Unmarshal([]byte(revisedSopJSON), &sopFile); err == nil && len(sopFile.SOPs) > 0 {
			parseSuccess = true
			log.Printf("Successfully parsed as complete SOPFile with %d SOPs", len(sopFile.SOPs))
			// 确保有question和analysis
			if sopFile.Question == "" {
				sopFile.Question = originalQuestion
			}
			if sopFile.Analysis == "" {
				sopFile.Analysis = gen.Messages[0].Thought
			}
		}
	}

	// 如果所有方式都失败，返回错误
	if !parseSuccess {
		return fmt.Errorf("failed to parse LLM response in any supported format. Content was:\n%s", revisedSopJSON)
	}

	// 验证最终结果
	if len(sopFile.SOPs) == 0 {
		return fmt.Errorf("parsed SOPFile has no SOPs")
	}

	log.Printf("Final SOPFile: Analysis length=%d, SOPs count=%d", len(sopFile.Analysis), len(sopFile.SOPs))

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

	evalLogPath := "../output/train_rep3.1_20250908103053.json"
	trainDataPath := "../../../../dataset/travelplanner/train/travelplanner_train_split.json"
	evaluationResultsPath := "../results/train_rep3.1_20250908103053_per_results_20250908.jsonl"
	// sopDir := "./gen_sop/"
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

		// sopPath := filepath.Join(sopDir, fmt.Sprintf("gen_sop_v3_q%d.json", result.ID))
		sopPath := filepath.Join(revisionOutDir, fmt.Sprintf("rev_rep_v3.1_q%d.json", result.ID))
		// sopPath := filepath.Join(fmt.Sprintf("repo/repo_sop_v3_q%d.json", result.ID))
		reflectionOutputPath := filepath.Join(reflectionOutDir, fmt.Sprintf("ref_rep_v3.1_q%d.json", result.ID))
		revisedSopPath := filepath.Join(revisionOutDir, fmt.Sprintf("rev_rep_v3.1.1_q%d.json", result.ID))

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
		if evalResult.Success {
			log.Printf("Query ID %d was successful according to evaluation. Copying successful SOP to rev_sop folder.", result.ID)
			successfulSopPath := revisedSopPath
			if err := os.WriteFile(successfulSopPath, sopBytes, 0644); err != nil {
				log.Printf("ERROR: Failed to copy successful SOP for query %d to %s: %v", result.ID, successfulSopPath, err)
			} else {
				log.Printf("Successfully copied SOP for query %d to %s", result.ID, successfulSopPath)
			}

			// Generate reflection file for successful case
			historyString := historyStrings[i]
			if err := generateSuccessfulReflection(string(sopBytes), historyString, question, reflectionOutputPath); err != nil {
				log.Printf("ERROR: Failed to generate reflection for successful query %d: %v", result.ID, err)
			}
			continue
		}

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

// v3 -> rev3 -> rev3.1 -> rev3.1.1 -> rev3.1.1.1
