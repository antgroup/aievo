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

// MBPPQuestion represents a single question from the MBPP dataset (train/valid/test JSONL)
type MBPPQuestion struct {
	ID          int      `json:"id"`
	TaskID      int      `json:"task_id"`
	Prompt      string   `json:"prompt"`
	Code        string   `json:"code"`
	Test        string   `json:"test"`
	EntryPoint  string   `json:"entry_point"`
	TestImports []string `json:"test_imports"`
	TestList    []string `json:"test_list"`
}

// Minimal message structure extracted from logs
type MinimalMessage struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Content  string `json:"content"`
}

// MBPPResultLog represents the execution result for a single MBPP task (from output/*.json)
type MBPPResultLog struct {
	ID                   int              `json:"id"`
	Query                string           `json:"query"`
	ModelOutput          string           `json:"model_output"`
	CommunicationHistory []MinimalMessage `json:"communication_history"`
	TotalCount           int              `json:"total_count"`
	Time                 string           `json:"time"`
}

// MBPPEvaluationResult represents the evaluation result with test outcomes (from results/*_results.jsonl)
type MBPPEvaluationResult struct {
	ID          int    `json:"id"`
	TaskID      int    `json:"task_id"`
	Status      string `json:"status"` // "pass" | "fail"
	Error       string `json:"error"`
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

// loadMBPPDataset loads the MBPP dataset from a JSONL file and returns a map by id
func loadMBPPDataset(filePath string) (map[int]MBPPQuestion, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	questions := make(map[int]MBPPQuestion)
	scanner := bufio.NewScanner(file)
	i := 0
	for scanner.Scan() {
		var question MBPPQuestion
		if err := json.Unmarshal(scanner.Bytes(), &question); err != nil {
			return nil, err
		}
		// Prefer explicit ID from file if present; fallback to index
		id := question.ID
		if id == 0 && question.TaskID == 0 {
			id = i
		}
		questions[id] = question
		i++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return questions, nil
}

// loadEvaluationResults reads the JSONL evaluation results file for MBPP
func loadEvaluationResults(filePath string) (map[int]MBPPEvaluationResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open evaluation file %s: %w", filePath, err)
	}
	defer file.Close()

	results := make(map[int]MBPPEvaluationResult)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var tmp struct {
			ID     int    `json:"id"`
			TaskID int    `json:"task_id"`
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &tmp); err != nil {
			log.Printf("Warning: failed to unmarshal evaluation result line: %v", err)
			continue
		}
		evalResult := MBPPEvaluationResult{
			ID:      tmp.ID,
			TaskID:  tmp.TaskID,
			Status:  tmp.Status,
			Error:   tmp.Error,
			Success: strings.EqualFold(tmp.Status, "pass"),
			TestResults: func() string {
				if tmp.Error != "" {
					return tmp.Error
				}
				if strings.EqualFold(tmp.Status, "pass") {
					return "pass"
				}
				return tmp.Status
			}(),
		}
		results[evalResult.ID] = evalResult
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading evaluation file: %w", err)
	}

	return results, nil
}

// filterCommunicationHistory filters the communication history to only include Sender, Receiver, and Content
func filterCommunicationHistory(messages []MinimalMessage) []map[string]string {
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

func performReflection(client llm.LLM, sopContent string, historyString string, question MBPPQuestion, evalResult MBPPEvaluationResult, modelOutput string, outputPath string) error {
	log.Printf("Performing reflection for id: %d, task_id: %d", question.ID, question.TaskID)

	constraintInfo := map[string]interface{}{
		"test_results": evalResult.TestResults,
		"success":      evalResult.Success,
	}
	constraintBytes, err := json.MarshalIndent(constraintInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal constraint information: %w", err)
	}
	constraintString := string(constraintBytes)

	// ReflectionPrompt parameters order (see prompt.go):
	// 1 user's problem, 2 standard answer, 3 system code, 4 evaluation results, 5 SOP used, 6 communication history
	prompt := fmt.Sprintf(ReflectionPrompt,
		question.Prompt,
		question.Code,
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

	// Try new-format SOP file, then old-format []SOP
	var sopFile SOPFile
	var workflowStr string
	if err := json.Unmarshal([]byte(sopContent), &sopFile); err == nil && len(sopFile.SOPs) > 0 {
		workflowStr = sopFile.SOPs[0].Workflow
	} else {
		var sops []SOP
		if err := json.Unmarshal([]byte(sopContent), &sops); err == nil && len(sops) > 0 {
			workflowStr = sops[0].Workflow
		} else {
			log.Printf("Warning: could not parse original SOP content as SOPFile or []SOP")
		}
	}

	outputData := ReflectionOutput{
		Question:      question.Prompt + "\nExample: " + question.TestList[0],
		OriginalSOP:   workflowStr,
		HistoryString: historyString,
		LLMReflection: llmReflection,
		GroundTruth:   question.Code,
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

func performRevision(client llm.LLM, questionPrompt string, originalSopBytes []byte, reflectionBytes []byte, outputPath string) error {
	log.Printf("Performing revision for SOP: %s", outputPath)

	// Use a MBPP SOP template present in this directory (e.g., v2.json)
	template_path := "v0.json"
	templateBytes, _ := os.ReadFile(template_path)
	var sopTemplate SOP
	if err := json.Unmarshal(templateBytes, &sopTemplate); err != nil {
		// try as array
		var sopArr []SOP
		if err2 := json.Unmarshal(templateBytes, &sopArr); err2 != nil || len(sopArr) == 0 {
			return fmt.Errorf("failed to unmarshal SOP template from %s: %v | %v", template_path, err, err2)
		}
		sopTemplate = sopArr[0]
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

	// Build section **1. User's Query and Original SOP:**
	userAndSOP := fmt.Sprintf("Question:\n%s\nOriginal SOP:\n%s", questionPrompt, originalSopContent)

	// Build section **3. Standard Answer for the User's Query:** from reflection (ground truth)
	standardAnswer := reflectionInput.GroundTruth
	if strings.TrimSpace(standardAnswer) == "" {
		standardAnswer = "" // keep empty but present
	}

	prompt := fmt.Sprintf(RevisionPrompt,
		templateString,
		userAndSOP,
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

	// The RevisionPrompt requires a wrapper with fields: thought, content (SOP), cate
	var wrapper struct {
		Thought string          `json:"thought"`
		Content json.RawMessage `json:"content"`
		Cate    string          `json:"cate"`
	}

	// Try to parse wrapper first
	var sopFile SOPFile
	if err := json.Unmarshal([]byte(revisedSopJSON), &wrapper); err == nil && len(wrapper.Content) > 0 && string(wrapper.Content) != "null" {
		var sop SOP
		// content might be a single SOP or an array with one
		if err2 := json.Unmarshal(wrapper.Content, &sop); err2 != nil {
			var sopArr []SOP
			if err3 := json.Unmarshal(wrapper.Content, &sopArr); err3 == nil && len(sopArr) > 0 {
				sop = sopArr[0]
			} else {
				return fmt.Errorf("failed to parse SOP content from wrapper: %v | %v\ncontent: %s", err2, err3, string(wrapper.Content))
			}
		}
		sopFile = SOPFile{
			Question: questionPrompt,
			Analysis: "",
			SOPs:     []SOP{sop},
		}
	} else {
		// fallback: try to extract `content` field generically, then parse SOP or []SOP
		var generic map[string]json.RawMessage
		if err2 := json.Unmarshal([]byte(revisedSopJSON), &generic); err2 == nil {
			if c, ok := generic["content"]; ok && len(c) > 0 && string(c) != "null" {
				var sop SOP
				if err3 := json.Unmarshal(c, &sop); err3 != nil {
					var sopArr []SOP
					if err4 := json.Unmarshal(c, &sopArr); err4 == nil && len(sopArr) > 0 {
						sop = sopArr[0]
					} else {
						return fmt.Errorf("failed to parse SOP from generic content: %v | %v\ncontent: %s", err3, err4, string(c))
					}
				}
				sopFile = SOPFile{Question: questionPrompt, Analysis: "", SOPs: []SOP{sop}}
			} else {
				// no content field: try direct SOP or []SOP
				var sop SOP
				if err3 := json.Unmarshal([]byte(revisedSopJSON), &sop); err3 == nil && (len(sop.Team) > 0 || sop.Workflow != "" || len(sop.Details) > 0) {
					sopFile = SOPFile{Question: questionPrompt, Analysis: "", SOPs: []SOP{sop}}
				} else {
					var sopArr []SOP
					if err4 := json.Unmarshal([]byte(revisedSopJSON), &sopArr); err4 == nil && len(sopArr) > 0 {
						sopFile = SOPFile{Question: questionPrompt, Analysis: "", SOPs: []SOP{sopArr[0]}}
					} else {
						return fmt.Errorf("failed to parse LLM response as SOP. wrapperErr=%v genericErr=%v sopErr=%v sopArrErr=%v. Content was:\n%s", err, err2, err3, err4, revisedSopJSON)
					}
				}
			}
		} else {
			// as a last resort, try direct SOP or []SOP
			var sop SOP
			if err3 := json.Unmarshal([]byte(revisedSopJSON), &sop); err3 == nil && (len(sop.Team) > 0 || sop.Workflow != "" || len(sop.Details) > 0) {
				sopFile = SOPFile{Question: questionPrompt, Analysis: "", SOPs: []SOP{sop}}
			} else {
				var sopArr []SOP
				if err4 := json.Unmarshal([]byte(revisedSopJSON), &sopArr); err4 == nil && len(sopArr) > 0 {
					sopFile = SOPFile{Question: questionPrompt, Analysis: "", SOPs: []SOP{sopArr[0]}}
				} else {
					return fmt.Errorf("failed to parse LLM response as SOP (no generic). wrapperErr=%v directSopErr=%v directArrErr=%v. Content was:\n%s", err, err3, err4, revisedSopJSON)
				}
			}
		}
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
	// --- CONFIGURATION (MBPP) ---
	// Prefer latest generated files in MBPP/output and MBPP/results
	outputDir := filepath.Join("..", "output")
	resultsDir := filepath.Join("..", "results")
	// Default to latest "train_*.json" or "test_*.json" under output
	// evalLogPath := findLatestFile(outputDir, []string{"train_*.json", "test_*.json", "valid*_*.json"})
	datasetPath := filepath.Join("../../../../dataset/MBPP", "mbpp_train.jsonl")
	filename := "train_qt_v2_20251119112800.json"
	evalLogPath := filepath.Join(outputDir, filename)
	evaluationResultsPath := filepath.Join(resultsDir, filename+"_results.jsonl")

	// sopDir := "./gen_sop/" // current SOP directory
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

	var results []MBPPResultLog
	if err := loadFile(evalLogPath, &results); err != nil {
		log.Fatalf("Error loading eval log: %v", err)
	}

	questions, err := loadMBPPDataset(datasetPath)
	if err != nil {
		log.Fatalf("Error loading MBPP dataset: %v", err)
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

		// if i < 20 {
		// 	continue
		// }

		// Choose a SOP file in this directory; prefer v2.json
		// sopPath := filepath.Join(sopDir, "v2.json")
		sopPath := filepath.Join("./gen_sop/", fmt.Sprintf("gen_sop_v2_q%d.json", result.ID))
		// sopPath := filepath.Join(revisionOutDir, fmt.Sprintf("rev_sop_v1_q%d.json", result.ID))
		reflectionOutputPath := filepath.Join(reflectionOutDir, fmt.Sprintf("ref_sop_v2_q%d.json", result.ID))
		revisedSopPath := filepath.Join(revisionOutDir, fmt.Sprintf("rev_sop_v2.1_q%d.json", result.ID))

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
			log.Printf("Query ID %d was successful according to evaluation. Copying successful SOP to rev_sop folder.", result.ID)
			successfulSopPath := revisedSopPath
			if err := os.WriteFile(successfulSopPath, sopBytes, 0644); err != nil {
				log.Printf("ERROR: Failed to copy successful SOP for query %d to %s: %v", result.ID, successfulSopPath, err)
			} else {
				log.Printf("Successfully copied SOP for query %d to %s", result.ID, successfulSopPath)
			}
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

		questionPrompt := question.Prompt + "\nExample: " + question.TestList[0]
		if err := performRevision(client, questionPrompt, sopBytes, reflectionBytes, revisedSopPath); err != nil {
			log.Printf("ERROR: Failed to perform revision for query %d: %v", i, err)
		}
	}

	log.Println("Revision process finished for MBPP.")
}
