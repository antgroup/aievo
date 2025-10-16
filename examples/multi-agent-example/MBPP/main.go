package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	"github.com/antgroup/aievo/tool/bash"
)

// MBPPQuestion represents a single question from the MBPP dataset
// Example fields reference: dataset/MBPP/mbpp_validate.jsonl | mbpp_test.jsonl
type MBPPQuestion struct {
	TaskID      int      `json:"task_id"`
	Prompt      string   `json:"prompt"`
	Code        string   `json:"code"`
	TestImports []string `json:"test_imports"`
	TestList    []string `json:"test_list"`
	EntryPoint  string   `json:"entry_point"`
	Test        string   `json:"test"`
}

// HumanEvalResultLog represents the evaluation result for a single question
type HumanEvalResultLog struct {
	ID                   int              `json:"id"`
	Query                string           `json:"query"`
	ModelOutput          string           `json:"model_output"`
	CommunicationHistory []schema.Message `json:"communication_history"`
	TotalCount           int              `json:"total_count"`
	Time                 string           `json:"time"`
}

// loadMBPPDataset loads the MBPP dataset from a JSONL file
func loadMBPPDataset(filePath string) ([]MBPPQuestion, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var questions []MBPPQuestion
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var question MBPPQuestion
		if err := json.Unmarshal(scanner.Bytes(), &question); err != nil {
			return nil, err
		}
		questions = append(questions, question)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return questions, nil
}

func createEvo(client llm.LLM, ts []tool.Tool, logFilePath string) (*aievo.AIEvo, error) {
	callbackHandler := &CallbackHandler{}

	// Instantiate Agents
	CoderA, _ := agent.NewBaseAgent(
		agent.WithName("CoderAgent"),
		agent.WithDesc(CoderAgentDescription),
		agent.WithPrompt(CoderAgentPrompt),
		agent.WithInstruction(defaultBaseInstructions),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
		agent.WithTools(ts),
		agent.WithSuffix(NULLSuffix),
		agent.WithLogFilePath(logFilePath),
	)

	AnswerA, _ := agent.NewBaseAgent(
		agent.WithName("AnswerAgent"),
		agent.WithDesc(AnswerAgentDescription),
		agent.WithPrompt(AnswerAgentPrompt),
		agent.WithInstruction(defaultEndBaseInstructions),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
		agent.WithSuffix(NULLSuffix),
		agent.WithLogFilePath(logFilePath),
	)

	env := environment.NewEnv()
	env.Memory = memory.NewBufferMemory()

	team := make([]schema.Agent, 0)
	team = append(team, CoderA, AnswerA)

	opts := make([]aievo.Option, 0)
	opts = append(opts,
		aievo.WithTeam(team),
		aievo.WithMaxTurn(10),
		aievo.WithCallback(callbackHandler),
		aievo.WithLLM(client),
		aievo.WithEnvironment(env),
		aievo.WithTeamLeader(CoderA),
		aievo.WithSOP(workflow),
		aievo.WithUserProxy(nil),
		aievo.WithSubMode(environment.ALLSubMode),
	)

	return aievo.NewAIEvo(opts...)
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

// SOPFile defines the structure of the generated SOP JSON file.
type SOPFile struct {
	Question string `json:"question"`
	Analysis string `json:"analysis"`
	SOPs     []SOP  `json:"sops"`
}

func createEvoFromSOP(client llm.LLM, ts []tool.Tool, sopPath string, sop *SOP, reflectionPath string, watcherInterval int, watcherActionInterval int, logFilePath string, maxWatcherUses int) (*aievo.AIEvo, error) {
	var selectedSOP SOP

	if sop != nil {
		selectedSOP = *sop
		log.Println("Successfully loaded SOP from parameter.")
	} else {
		sopFileBytes, err := os.ReadFile(sopPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SOP file: %w", err)
		}

		var sopFile SOPFile
		var sops []SOP
		if err := json.Unmarshal(sopFileBytes, &sopFile); err == nil && len(sopFile.SOPs) > 0 {
			sops = sopFile.SOPs
			log.Printf("Successfully loaded SOP from new file format (question: %s)", sopFile.Question)
		} else {
			if err := json.Unmarshal(sopFileBytes, &sops); err != nil {
				return nil, fmt.Errorf("failed to unmarshal SOP JSON in either new or old format: %w", err)
			}
			log.Println("Successfully loaded SOP from old file format.")
		}

		if len(sops) == 0 {
			return nil, fmt.Errorf("no SOPs found in the JSON file")
		}
		selectedSOP = sops[len(sops)-1]
	}

	callbackHandler := &CallbackHandler{}
	agentsMap := make(map[string]schema.Agent)
	var team []schema.Agent

	env := environment.NewEnv()
	env.Memory = memory.NewBufferMemory()

	for _, agentDetail := range selectedSOP.Details {
		desc := agentDetail.Responsibility

		agentOpts := []agent.Option{
			agent.WithName(agentDetail.Name),
			agent.WithDesc(desc),
			agent.WithPrompt(desc),
			agent.WithRole(agentDetail.Instruction),
			agent.WithLLM(client),
			agent.WithEnv(env),
			agent.WithCallback(callbackHandler),
			agent.WithSuffix(NULLSuffix),
			agent.WithReflectionPath(reflectionPath),
			agent.WithLogFilePath(logFilePath),
		}

		var selectedTools []tool.Tool
		for _, toolName := range agentDetail.Tools {
			if strings.EqualFold(toolName, "bash") {
				selectedTools = append(selectedTools, ts...)
				break
			}
		}
		if len(selectedTools) > 0 {
			agentOpts = append(agentOpts, agent.WithTools(selectedTools))
		}

		var instructionsToUse string
		if len(team)+1 == len(selectedSOP.Details) {
			instructionsToUse = NewEndBaseInstructions
		} else {
			instructionsToUse = NewBaseInstructions
		}
		agentOpts = append(agentOpts, agent.WithInstruction(instructionsToUse))

		newAgent, err := agent.NewBaseAgent(agentOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create agent %s: %w", agentDetail.Name, err)
		}
		agentsMap[agentDetail.Name] = newAgent
		team = append(team, newAgent)
	}

	var teamLeader schema.Agent
	if len(selectedSOP.Team) > 0 {
		teamLeader = agentsMap[selectedSOP.Team[0]]
	}

	// var watcherReflectionPath string
	// if len(reflectionPaths) > 0 {
	// watcherReflectionPath = reflectionPaths[0]
	// }
	watcherReflectionPath := reflectionPath
	watcher, _ := agent.NewWatcherAgent(
		agent.WithLLM(client),
		agent.WithEnv(env),
		agent.WithPrompt(WatchPrompt),
		agent.WithInstruction(WatchInstructions),
		agent.WithCallback(callbackHandler),
		agent.WithSuffix(WatchSuffix),
		agent.WithReflectionPath(watcherReflectionPath),
		agent.WithLogFilePath(logFilePath),
	)

	opts := []aievo.Option{
		aievo.WithTeam(team),
		aievo.WithMaxTurn(20),
		aievo.WithCallback(callbackHandler),
		aievo.WithLLM(client),
		aievo.WithEnvironment(env),
		aievo.WithTeamLeader(teamLeader),
		aievo.WithSOP(selectedSOP.Workflow),
		aievo.WithUserProxy(nil),
		aievo.WithSubMode(environment.ALLSubMode),
		aievo.WithWatcher(watcher, func(message schema.Message, memory schema.Memory, turn int) bool {
			messages := memory.Load(context.Background(), nil)
			msgCount := len(messages)
			return msgCount > 0 && msgCount%watcherInterval == 0
		}),
		aievo.WithWatcherInterval(watcherInterval),
		aievo.WithWatcherActionInterval(watcherActionInterval),
		aievo.WithMaxWatcherUses(maxWatcherUses),
	}

	return aievo.NewAIEvo(opts...)
}

func generateSOP(client llm.LLM, userQuestion, sopTemplatePath, newSopOutputPath string, logFilePath string, writeToFile bool) (*SOP, error) {
	log.Println("Starting SOP generation...")

	// 1. Load the SOP template file
	sopFileBytes, err := os.ReadFile(sopTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SOP template file: %w", err)
	}

	var prompt string

	// 2. Detect file format and prepare the prompt
	var sopFile SOPFile
	// Try to unmarshal into the new SOPFile structure first.
	if err := json.Unmarshal(sopFileBytes, &sopFile); err == nil && len(sopFile.SOPs) > 0 {
		log.Printf("Detected RAG-style SOP template from: %s", sopTemplatePath)

		// Extract example data for the RAG prompt
		exampleQuestion := sopFile.Question
		exampleAnalysis := sopFile.Analysis
		exampleSOP := sopFile.SOPs[len(sopFile.SOPs)-1]

		exampleSOPBytes, err := json.MarshalIndent(exampleSOP, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal example SOP to string: %w", err)
		}
		exampleSOPString := string(exampleSOPBytes)

		// 2.1.1 Use the RAG prompt with the extracted examples
		// exampleAnalysis := "{The analysis process. For brevity, it is omitted here.}"
		// prompt = fmt.Sprintf(SOPGeneratorPrompt_rag, exampleQuestion, exampleAnalysis, exampleSOPString, userQuestion)

		// 2.1.2 RAG + templete
		template_path := "SOP/v2.json"
		templateBytes, err := os.ReadFile(template_path)
		if err != nil {
			prompt = fmt.Sprintf(SOPGeneratorPrompt_rag, exampleQuestion, exampleAnalysis, exampleSOPString, userQuestion)
			log.Printf("Failed to read template file %s: %v. Using RAG prompt only.", template_path, err)
		} else {
			var sops []SOP
			if err := json.Unmarshal(templateBytes, &sops); err != nil {
				return nil, fmt.Errorf("failed to unmarshal SOP JSON in either new or old format: %w", err)
			}
			if len(sops) == 0 {
				return nil, fmt.Errorf("no SOPs found in the template file")
			}
			templateSOP := sops[len(sops)-1] // Get the last one as template

			templateBytes, err := json.MarshalIndent(templateSOP, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal SOP template to string: %w", err)
			}
			templateString := string(templateBytes)

			// exampleAnalysis = "{The analysis process. For brevity, it is omitted here.}"

			prompt = fmt.Sprintf(SOPGeneratorPrompt_temp_rag, templateString, exampleQuestion, exampleAnalysis, exampleSOPString, userQuestion)
		}

	} else { // 2.2 只使用模板SOP
		log.Printf("Detected standard SOP template from: %s", sopTemplatePath)
		// Fallback to the old format (an array of SOPs)
		var sops []SOP
		if err := json.Unmarshal(sopFileBytes, &sops); err != nil {
			return nil, fmt.Errorf("failed to unmarshal SOP JSON in either new or old format: %w", err)
		}
		if len(sops) == 0 {
			return nil, fmt.Errorf("no SOPs found in the template file")
		}
		templateSOP := sops[len(sops)-1] // Get the last one as template

		templateBytes, err := json.MarshalIndent(templateSOP, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal SOP template to string: %w", err)
		}
		templateString := string(templateBytes)

		// Use the standard prompt
		prompt = fmt.Sprintf(SOPGeneratorPrompt, templateString, userQuestion) // pmt_v4
	}

	// 3. Create a temporary agent to generate the SOP
	sopGenerator, err := agent.NewBaseAgent(
		agent.WithName("SOPGenerator"),
		agent.WithDesc("A specialized agent that generates a Standard Operating Procedure (SOP) for a multi-agent system based on a user's question and a template."),
		agent.WithPrompt(prompt),
		agent.WithLLM(client),
		agent.WithInstruction(""),
		agent.WithSuffix(NULLSuffix), // Use a null suffix
		agent.WithLogFilePath(logFilePath),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOPGenerator agent: %w", err)
	}

	// 4. Call LLM to generate the new SOP by running the agent
	log.Println("Calling LLM to generate new SOP...")
	gen, err := sopGenerator.Run(context.Background(), []schema.Message{
		{
			Type:     schema.MsgTypeMsg,
			Content:  "You are an expert in designing multi-agent systems.",
			Sender:   "User",
			Receiver: "SOPGenerator",
		},
	}, llm.WithTemperature(0.6), llm.WithTopP(0.95))
	if err != nil {
		return nil, fmt.Errorf("SOPGenerator agent run failed: %w", err)
	}

	if len(gen.Messages) == 0 || gen.Messages[0].Content == "" {
		return nil, fmt.Errorf("LLM returned an empty response")
	}

	agentResponse := gen.Messages[0]
	log.Printf("SOP Generator Thought: %s", agentResponse.Thought)

	// The actual SOP is in the 'Content' field.
	sopJSON := string(agentResponse.Content)

	// 6. Validate the new SOP
	var newSop SOP
	if err := json.Unmarshal([]byte(sopJSON), &newSop); err != nil {
		return nil, fmt.Errorf("failed to unmarshal generated SOP JSON from content field: %w. SOP JSON was: %s", err, sopJSON)
	}

	// 7. Save the new SOP to file if requested
	if writeToFile {
		// Create the new file structure
		outputFileContent := SOPFile{
			Question: userQuestion,
			Analysis: agentResponse.Thought,
			SOPs:     []SOP{newSop},
		}

		fileContent, err := json.MarshalIndent(outputFileContent, "", "  ")
		if err != nil {
			return &newSop, fmt.Errorf("failed to marshal new SOP file content: %w", err)
		}

		if err := os.WriteFile(newSopOutputPath, fileContent, 0644); err != nil {
			return &newSop, fmt.Errorf("failed to write new SOP to file: %w", err)
		}
		log.Printf("Successfully generated and saved new SOP to %s", newSopOutputPath)
	}

	return &newSop, nil
}

// retrieveSOPFile retrieves the top SOP number from the retrieval results.
func retrieveSOPFile(mode string, questionID int) (int, error) {
	var retrievalPath string
	switch mode {
	case "eval":
		retrievalPath = "Analysis/retri_results_valid_qwen.json"
	}

	retrievalFile, err := os.ReadFile(retrievalPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read retrieval file %s: %w", retrievalPath, err)
	}

	type RetrievalResult struct {
		WeightedSimilarity []int `json:"weighted_similarity"`
		AnalysisSimilarity []int `json:"analysis_similarity"`
	}
	type RetrievalEntry struct {
		ID               int             `json:"id"`
		RetrievalResults RetrievalResult `json:"retrieval_results"`
	}
	var retrievalData []RetrievalEntry
	if err := json.Unmarshal(retrievalFile, &retrievalData); err != nil {
		return 0, fmt.Errorf("failed to parse retrieval file %s: %w", retrievalPath, err)
	}

	for _, entry := range retrievalData {
		if entry.ID == questionID {
			//if len(entry.RetrievalResults.WeightedSimilarity) > 0 {
			//	return entry.RetrievalResults.WeightedSimilarity[0], nil
			//}
			if len(entry.RetrievalResults.AnalysisSimilarity) > 0 {
				return entry.RetrievalResults.AnalysisSimilarity[0], nil
			}
			return 0, fmt.Errorf("found entry for question ID %d, but weighted_similarity is empty", questionID)
		}
	}

	return 0, fmt.Errorf("could not find entry for question ID %d in %s", questionID, retrievalPath)
}

func main() {
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		// openai.WithModel("Qwen2.5-72B-Instruct"),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}

	bashTool, err := bash.New()
	if err != nil {
		log.Fatalf("Failed to create BashTool: %v", err)
	}
	tools := []tool.Tool{bashTool}

	var datasetPath string
	var mode string
	eval := 2
	switch eval {
	case 0:
		// MBPP doesn't have a standard train split here; reuse validate for development
		mode = "train"
		datasetPath = "../../../dataset/MBPP/mbpp_validate.jsonl"
	case 1:
		mode = "eval"
		datasetPath = "../../../dataset/MBPP/mbpp_validate.jsonl"
	case 2:
		mode = "test"
		datasetPath = "../../../dataset/MBPP/mbpp_test.jsonl"
	}

	var results []HumanEvalResultLog
	totalCount := 0
	timeStamp := time.Now().Format("20060102150405")
	resultsFilename := fmt.Sprintf("output/%s_t0_%s.json", mode, timeStamp)
	ErrorlogFilename := strings.TrimSuffix(resultsFilename, ".json") + ".log"
	logFilename := "log/" + strings.TrimSuffix(resultsFilename[7:], ".json") + ".log"
	start_time := time.Now()
	start_id := 0
	// end_id := 22 //len(questions)
	watcherInterval := 30
	watcherActionInterval := 50
	maxWatcherUses := 2 // 设置watcher最大使用次数
	// test_id := []int{287, 288, 300, 301}

	fmt.Printf("\n################## Starting Evaluation for MBPP ##################\n")
	fmt.Printf("Loading dataset from: %s\n", datasetPath)

	questions, err := loadMBPPDataset(datasetPath)
	if err != nil {
		log.Printf("Failed to load MBPP dataset, exiting: %v", err)
		return
	}

	for i, q := range questions {

		// test test_id_set
		// testIDSet := make(map[int]struct{})
		// for _, id := range test_id {
		// 	testIDSet[id] = struct{}{}
		// }
		// if _, found := testIDSet[i]; !found {
		// 	continue
		// }

		if i < start_id {
			continue
		}

		question := q.Prompt

		fromsop := true
		var evo *aievo.AIEvo
		var err error
		var generateNewSOP bool

		fmt.Printf("\n==================Processing question ID: %d\n", i)
		totalCount++
		logFile, logErr := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if logErr == nil {
			defer logFile.Close()
			logEntry := fmt.Sprintf("\n\n===============Processing question ID: %d\n", i)
			logFile.WriteString(logEntry)
		}

		if fromsop {
			sopPath := "SOP/v0.json"
			if eval == 0 {
				generateNewSOP = false //
			} else {
				generateNewSOP = true // For eval set, true to enable generation
			}
			if generateNewSOP { // 评估集：LLM生成SOP
				newSopPath := fmt.Sprintf("SOP/val_sop/gen_sop_v2_q%d.json", i)
				reflectionPath := ""
				// Set writeToFile to true if you want to save the generated SOP.
				writeToFile := false
				rag := true
				if rag { // RAG模式：从检索SOP作为引导生成SOP
					retrievedQuestionNumber, err := retrieveSOPFile(mode, i)
					if err != nil {
						log.Printf("WARNING: RAG mode failed to retrieve SOP file: %v. Falling back to default SOP.", err)
					} else {
						// retrievedSopPath := fmt.Sprintf("SOP/gen_sop/gen_sop_v3_q%d.json", retrievedQuestionNumber)
						retrievedSopPath := fmt.Sprintf("SOP/repo/repo_sop_v1_q%d.json", retrievedQuestionNumber)
						log.Printf("RAG mode: refer to retrieved SOP: %s", retrievedSopPath)
						sopPath = retrievedSopPath
						// reflectionPath = fmt.Sprintf("SOP/reflect/ref_rep_v3_q%d.json", retrievedQuestionNumber)
					}
				} // 依据通用模板 / rag 生成SOP
				generatedSOP, err := generateSOP(client, question, sopPath, newSopPath, logFilename, writeToFile)
				if err != nil {
					log.Printf("ERROR: Failed to generate SOP for question %d, falling back to default: %v", i, err)
					// Fallback to default SOP if generation fails
					evo, err = createEvoFromSOP(client, tools, sopPath, nil, reflectionPath, watcherInterval, watcherActionInterval, logFilename, maxWatcherUses)
					if err != nil {
						panic(err)
					}
				} else {
					log.Printf("Using generated SOP for question %d", i)
					evo, err = createEvoFromSOP(client, tools, "", generatedSOP, reflectionPath, watcherInterval, watcherActionInterval, logFilename, maxWatcherUses)
					if err != nil {
						panic(err)
					}
				}
			} else { // 训练集：不生成SOP，直接使用已有的SOP
				// sopPath = fmt.Sprintf("SOP/rev_sop/rev_rep_v3.1_q%d.json", i)
				reflectionPath := ""
				// sopPath = fmt.Sprintf("SOP/gen_sop/gen_sop_v3_q%d.json", i)
				// sopPath = fmt.Sprintf("SOP/repo/repo_sop_v1_q%d.json", i)
				evo, err = createEvoFromSOP(client, tools, sopPath, nil, reflectionPath, watcherInterval, watcherActionInterval, logFilename, maxWatcherUses)

				// newSopPath := fmt.Sprintf("SOP/gen_sop/gen_sop_v3_q%d.json", i)
				// writeToFile := true // 训练集：生成SOP并写入文件
				// generatedSOP, err := generateSOP(client, question, sopPath, newSopPath, logFilename, writeToFile)
				// // generatedSOP, err := generateSOP_train(client, question, q.AnnotatedPlan, sopPath, newSopPath, writeToFile)
				// if err != nil {
				// 	log.Printf("ERROR: Failed to generate SOP for question %d, falling back to default: %v", i, err)
				// 	// Fallback to default SOP if generation fails
				// 	evo, err = createEvoFromSOP(client, tools, sopPath, nil, "", watcherInterval, watcherActionInterval, logFilename, maxWatcherUses)
				// 	if err != nil {
				// 		panic(err)
				// 	}
				// } else {
				// 	log.Printf("Using generated SOP for question %d", i)
				// 	// Use the generated SOP for the current question
				// 	evo, err = createEvoFromSOP(client, tools, "", generatedSOP, "", watcherInterval, watcherActionInterval, logFilename, maxWatcherUses)
				// 	if err != nil {
				// 		panic(err)
				// 	}
				// }
			}
		} else { // 手动构建团队
			evo, err = createEvo(client, tools, logFilename)
		}
		if err != nil {
			panic(fmt.Errorf("failed to create AIEvo instance: %w", err))
		}

		gen, err := evo.Run(context.Background(), question,
			llm.WithTemperature(0.6), llm.WithTopP(0.95))
		if err != nil {
			log.Printf("Error running engineer for query: %v", err)
			logFile, logErr := os.OpenFile(ErrorlogFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if logErr == nil {
				defer logFile.Close()
				logEntry := fmt.Sprintf("-----ID: %d  Task ID:%v\n---Query:%s\n---Error: %v\n\n", i, q.TaskID, q.Prompt, err)
				logFile.WriteString(logEntry)
			}
			gen = "NULL"
		}

		fmt.Printf("Model Output Answer: %s\n", gen)

		var communicationHistory []schema.Message
		if buffer, ok := evo.Environment.Memory.(*memory.Buffer); ok {
			communicationHistory = buffer.Messages
		}

		modelOutputContent := gen

		results = append(results, HumanEvalResultLog{
			ID:                   i,
			Query:                q.Prompt,
			ModelOutput:          modelOutputContent,
			CommunicationHistory: communicationHistory,
			TotalCount:           totalCount,
			Time:                 time.Since(start_time).String(),
		})

		resultsJSON, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal results to JSON: %v", err)
		}

		err = os.WriteFile(resultsFilename, resultsJSON, 0644)
		if err != nil {
			log.Fatalf("Failed to write results to file: %v", err)
		}

		fmt.Printf("\n===========Total Count: %d\n", totalCount)
	}

	fmt.Printf("\nEvaluation finished for MBPP. Results saved to %s\n", resultsFilename)
}
