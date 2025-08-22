package main

import (
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
	"github.com/antgroup/aievo/tool/travel"
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

// loadTravelPlannerDataset loads the TravelPlanner dataset from a JSON file
func loadTravelPlannerDataset(filePath string) ([]TravelPlannerQuestion, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var questions []TravelPlannerQuestion
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
		agent.WithTools(ts),
		agent.WithSuffix(NULLSuffix),
	)

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
	team = append(team, PlanA, AnswerA)

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

type SOP struct {
	Team    []string      `json:"team"`
	SOP     string        `json:"sop"`
	Details []AgentDetail `json:"details"`
}

type AgentDetail struct {
	Name           string   `json:"name"`
	Responsibility string   `json:"responsibility"` // v4: role
	Instruction    string   `json:"instruction"`
	Tools          []string `json:"tools"`
}

// SOPFile defines the structure of the generated SOP JSON file.
type SOPFile struct {
	Question string `json:"question"`
	Analysis string `json:"analysis"`
	SOPs     []SOP  `json:"sops"`
}

func createEvoFromSOP(client llm.LLM, ts []tool.Tool, sopPath string, sop *SOP, reflectionPath string, watcherInterval int) (*aievo.AIEvo, error) {
	var selectedSOP SOP

	if sop != nil {
		selectedSOP = *sop
		log.Println("Successfully loaded SOP from parameter.")
	} else {
		sopFileBytes, err := os.ReadFile(sopPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SOP file: %w", err)
		}

		// Try to unmarshal into the new SOPFile structure first.
		var sopFile SOPFile
		var sops []SOP
		if err := json.Unmarshal(sopFileBytes, &sopFile); err == nil && len(sopFile.SOPs) > 0 {
			sops = sopFile.SOPs
			log.Printf("Successfully loaded SOP from new file format (question: %s)", sopFile.Question)
		} else {
			// Fallback to the old format (an array of SOPs)
			if err := json.Unmarshal(sopFileBytes, &sops); err != nil {
				return nil, fmt.Errorf("failed to unmarshal SOP JSON in either new or old format: %w", err)
			}
			log.Println("Successfully loaded SOP from old file format.")
		}

		if len(sops) == 0 {
			return nil, fmt.Errorf("no SOPs found in the JSON file")
		}
		selectedSOP = sops[len(sops)-1] // Get the last SOP as the selected one
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
		}

		// Create a slice to store selected tools for this agent
		var selectedTools []tool.Tool

		for _, toolName := range agentDetail.Tools {
			if strings.EqualFold(toolName, "search") {
				selectedTools = ts
				break
			}
			// Map tool names to specific tools from ts array
			switch toolName {
			case "FlightSearch":
				selectedTools = append(selectedTools, ts[0]) // FlightTool
			case "AccommodationSearch":
				selectedTools = append(selectedTools, ts[1]) // AccommodationTool
			case "RestaurantSearch":
				selectedTools = append(selectedTools, ts[2]) // RestaurantTool
			case "AttractionSearch":
				selectedTools = append(selectedTools, ts[3]) // AttractionTool
			case "GoogleDistanceMatrix":
				selectedTools = append(selectedTools, ts[4]) // DistanceTool
			case "CitySearch":
				selectedTools = append(selectedTools, ts[5]) // CityTool
			case "CostEnquiry":
				selectedTools = append(selectedTools, ts[6]) // CostEnquiryTool
			}
		}
		// Only add tools if we found any matches
		if len(selectedTools) > 0 {
			agentOpts = append(agentOpts, agent.WithTools(selectedTools))
		}

		// Check if this is the last agent in the team
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

	// Use WatcherAgent
	// watcher, _ := agent.NewWatcherAgent(
	// 	agent.WithLLM(client),
	// 	agent.WithEnv(env),
	// 	agent.WithPrompt(WatchPrompt),
	// 	agent.WithInstruction(WatchInstructions),
	// 	agent.WithCallback(callbackHandler),
	// 	agent.WithSuffix(WatchSuffix),
	// 	agent.WithReflectionPath(reflectionPath),
	// )

	opts := []aievo.Option{
		aievo.WithTeam(team),
		aievo.WithMaxTurn(20),
		aievo.WithCallback(callbackHandler),
		aievo.WithLLM(client),
		aievo.WithEnvironment(env),
		aievo.WithTeamLeader(teamLeader),
		aievo.WithSOP(selectedSOP.SOP),
		aievo.WithUserProxy(nil),
		aievo.WithSubMode(environment.ALLSubMode),
		// aievo.WithWatcher(watcher, func(message schema.Message, memory schema.Memory) bool {
		// 	messages := memory.Load(context.Background(), nil)
		// 	msgCount := len(messages)
		// 	return msgCount > 0 && msgCount%watcherInterval == 0
		// }),
		// aievo.WithWatcherInterval(watcherInterval),
	}

	return aievo.NewAIEvo(opts...)
}

func generateSOP(client llm.LLM, userQuestion, sopTemplatePath, newSopOutputPath string, writeToFile bool) (*SOP, error) {
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
		// prompt = fmt.Sprintf(SOPGeneratorPrompt_rag, exampleQuestion, exampleAnalysis, exampleSOPString, userQuestion)

		// 2.1.2 RAG + templete
		template_path := "SOP/v6.json"
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

func generateSOP_train(client llm.LLM, userQuestion string, metadata string, sopTemplatePath, newSopOutputPath string, writeToFile bool) (*SOP, error) {
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

		// Use the RAG prompt with the extracted examples
		prompt = fmt.Sprintf(SOPGeneratorPrompt_rag, exampleQuestion, exampleAnalysis, exampleSOPString, userQuestion)

	} else {
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
		prompt = fmt.Sprintf(SOPGeneratorPrompt_train, templateString, userQuestion, metadata)
	}

	// 3. Create a temporary agent to generate the SOP
	sopGenerator, err := agent.NewBaseAgent(
		agent.WithName("SOPGenerator"),
		agent.WithDesc("A specialized agent that generates a Standard Operating Procedure (SOP) for a multi-agent system based on a user's question and a template."),
		agent.WithPrompt(prompt),
		agent.WithLLM(client),
		agent.WithInstruction(""),
		agent.WithSuffix(NULLSuffix), // Use a null suffix
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

// retrieveSOPFile retrieves the top SOP filename from the retrieval results.
func retrieveSOPFile(questionID int) (string, error) {
	retrievalPath := "Analysis/retri_results_bge.json" // bge or qwen
	retrievalFile, err := os.ReadFile(retrievalPath)
	if err != nil {
		return "", fmt.Errorf("failed to read retrieval file %s: %w", retrievalPath, err)
	}

	type RetrievalResult struct {
		WeightedSimilarity []string `json:"weighted_similarity"`
		AnalysisSimilarity []string `json:"analysis_similarity"`
	}
	type RetrievalEntry struct {
		ID               int             `json:"id"`
		RetrievalResults RetrievalResult `json:"retrieval_results"`
	}
	var retrievalData []RetrievalEntry
	if err := json.Unmarshal(retrievalFile, &retrievalData); err != nil {
		return "", fmt.Errorf("failed to parse retrieval file %s: %w", retrievalPath, err)
	}

	for _, entry := range retrievalData {
		if entry.ID == questionID {
			if len(entry.RetrievalResults.WeightedSimilarity) > 0 {
				return entry.RetrievalResults.WeightedSimilarity[0], nil
			}
			// if len(entry.RetrievalResults.AnalysisSimilarity) > 0 {
			// return entry.RetrievalResults.AnalysisSimilarity[0], nil
			// }
			return "", fmt.Errorf("found entry for question ID %d, but weighted_similarity is empty", questionID)
		}
	}

	return "", fmt.Errorf("could not find entry for question ID %d in %s", questionID, retrievalPath)
}

func main() {
	// 大模型实例化
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		// openai.WithModel("Qwen2.5-72B-Instruct"),
		// openai.WithModel("Qwen3-235B-A22B-Thinking-2507"),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}
	// 实例化所需要的Tools

	tools := []tool.Tool{}

	// Travel Tools
	flightTool, err := travel.NewFlightTool(travel.WithDatabasePath("../../../dataset/travelplanner/database"))
	if err != nil {
		log.Printf("Failed to create FlightTool: %v", err)
	} else {
		tools = append(tools, flightTool)
	}
	accommodationTool, err := travel.NewAccommodationTool(travel.WithDatabasePath("../../../dataset/travelplanner/database"))
	if err != nil {
		log.Printf("Failed to create AccommodationTool: %v", err)
	} else {
		tools = append(tools, accommodationTool)
	}
	restaurantTool, err := travel.NewRestaurantTool(travel.WithDatabasePath("../../../dataset/travelplanner/database"))
	if err != nil {
		log.Printf("Failed to create RestaurantTool: %v", err)
	} else {
		tools = append(tools, restaurantTool)
	}
	attractionTool, err := travel.NewAttractionTool(travel.WithDatabasePath("../../../dataset/travelplanner/database"))
	if err != nil {
		log.Printf("Failed to create AttractionTool: %v", err)
	} else {
		tools = append(tools, attractionTool)
	}
	distanceTool, err := travel.NewDistanceTool(travel.WithDatabasePath("../../../dataset/travelplanner/database"))
	if err != nil {
		log.Printf("Failed to create DistanceTool: %v", err)
	} else {
		tools = append(tools, distanceTool)
	}
	cityTool, err := travel.NewCityTool(travel.WithDatabasePath("../../../dataset/travelplanner/database"))
	if err != nil {
		log.Printf("Failed to create CityTool: %v", err)
	} else {
		tools = append(tools, cityTool)
	}
	costEnquiryTool, err := travel.NewCostEnquiryTool(travel.WithDatabasePath("../../../dataset/travelplanner/database"))
	if err != nil {
		log.Printf("Failed to create CostEnquiryTool: %v", err)
	} else {
		tools = append(tools, costEnquiryTool)
	}

	var mode string
	datasetPath := ""
	eval := 0 // 0 for training, 2 for evaluation
	if eval == 0 {
		mode = "train"
		datasetPath = "../../../dataset/travelplanner/train/travelplanner_train_split.json"
	} else if eval == 1 {
		mode = "eval"
		datasetPath = "../../../dataset/travelplanner/train/travelplanner_eval_split.json"
	} else if eval == 2 {
		mode = "validation"
		datasetPath = "../../../dataset/travelplanner/validation/travelplanner_validation_dataset.json"
	} else if eval == 3 {
		mode = "train"
		datasetPath = "../../../dataset/travelplanner/train/travelplanner_train_dataset.json"
	}

	fmt.Printf("\n################## Starting Evaluation for TravelPlanner ##################\n")
	fmt.Printf("Loading dataset from: %s\n", datasetPath)

	questions, err := loadTravelPlannerDataset(datasetPath)
	if err != nil {
		log.Printf("Failed to load TravelPlanner dataset, exiting: %v", err)
		return
	}

	var results []TravelPlannerResultLog
	totalCount := 0
	timeStamp := time.Now().Format("20060102150405")
	resultsFilename := fmt.Sprintf("output/%s_t1.1_%s.json", mode, timeStamp)
	logFilename := strings.TrimSuffix(resultsFilename, ".json") + ".log"
	start_time := time.Now()
	start_id := 0
	// end_id := 3 //len(questions)
	watcherInterval := 50

	for i, q := range questions {
		if i < start_id {
			continue
		}
		// if i >= end_id {
		// break
		// }

		question := q.Query

		fromsop := true
		var evo *aievo.AIEvo
		var err error
		var generateNewSOP bool

		fmt.Printf("\n==================Processing question ID: %d\n", i)
		totalCount++

		if fromsop {
			sopPath := "SOP/v1.json"
			if eval == 0 {
				generateNewSOP = false //
			} else {
				generateNewSOP = false // For eval set, true to enable generation
			}
			if generateNewSOP { // 评估集：LLM生成SOP
				newSopPath := fmt.Sprintf("SOP/val_sop/gen_sop_v1_q%d.json", i)
				reflectionPath := ""
				// Set writeToFile to true if you want to save the generated SOP.
				writeToFile := false
				rag := true
				if rag { // RAG模式：从检索SOP作为引导生成SOP
					retrievedSopFile, err := retrieveSOPFile(i)
					if err != nil {
						log.Printf("WARNING: RAG mode failed to retrieve SOP file: %v. Falling back to default SOP.", err)
					} else {
						questionNumber := string(retrievedSopFile[len(retrievedSopFile)-6])
						retrievedSopFile = fmt.Sprintf("gen_sop_v6_q%s.json", questionNumber)

						retrievedSopPath := fmt.Sprintf("SOP/gen_sop/%s", retrievedSopFile)
						log.Printf("RAG mode: refer to retrieved SOP: %s", retrievedSopPath)
						sopPath = retrievedSopPath

						reflectionPath = fmt.Sprintf("SOP/reflect/ref_v6.1_q%s.json", questionNumber)
					}
				} // 依据通用模板 / rag 生成SOP
				generatedSOP, err := generateSOP(client, question, sopPath, newSopPath, writeToFile)
				if err != nil {
					log.Printf("ERROR: Failed to generate SOP for question %d, falling back to default: %v", i, err)
					// Fallback to default SOP if generation fails
					evo, err = createEvoFromSOP(client, tools, sopPath, nil, reflectionPath, watcherInterval)
					if err != nil {
						panic(err)
					}
				} else {
					log.Printf("Using generated SOP for question %d", i)
					evo, err = createEvoFromSOP(client, tools, "", generatedSOP, reflectionPath, watcherInterval)
					if err != nil {
						panic(err)
					}
				}
			} else { // 训练集：不生成SOP，直接使用已有的SOP
				// sopPath = fmt.Sprintf("SOP/rev_sop/rev_sop_v1_L0_q%d.json", i)
				reflectionPath := fmt.Sprintf("SOP/reflect/ref_v1_q%d.json", i)
				//sopPath = fmt.Sprintf("SOP/gen_sop/gen_sop_v1_L%d_q%d.json", level, i)
				evo, err = createEvoFromSOP(client, tools, sopPath, nil, reflectionPath, watcherInterval)

				// newSopPath := fmt.Sprintf("SOP/gen_sop/gen_sop_v1_q%d.json", i)
				// writeToFile := true // 训练集：生成SOP并写入文件
				// generatedSOP, err := generateSOP(client, question, sopPath, newSopPath, writeToFile)
				// // generatedSOP, err := generateSOP_train(client, question, q.AnnotatedPlan, sopPath, newSopPath, writeToFile)
				// if err != nil {
				// 	log.Printf("ERROR: Failed to generate SOP for question %d, falling back to default: %v", i, err)
				// 	// Fallback to default SOP if generation fails
				// 	evo, err = createEvoFromSOP(client, tools, sopPath, nil, "", watcherInterval)
				// 	if err != nil {
				// 		panic(err)
				// 	}
				// } else {
				// 	log.Printf("Using generated SOP for question %d", i)
				// 	// Use the generated SOP for the current question
				// 	evo, err = createEvoFromSOP(client, tools, "", generatedSOP, "", watcherInterval)
				// 	if err != nil {
				// 		panic(err)
				// 	}
				// }
			}
		} else { // 手动构建团队
			evo, err = createEvo(client, tools)
		}
		if err != nil {
			panic(fmt.Errorf("failed to create AIEvo instance: %w", err))
		}

		gen, err := evo.Run(context.Background(), question,
			llm.WithTemperature(0.6), llm.WithTopP(0.95))
		if err != nil {
			log.Printf("Error running engineer for query: %v", err)
			// 记录错误信息到log文件
			logFile, logErr := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if logErr == nil {
				defer logFile.Close()
				logEntry := fmt.Sprintf("-----ID: %d\n---Query:%s\n---Error: %v\n\n", i, q.Query, err)
				logFile.WriteString(logEntry)
			}
			gen = "NULL"
		}

		// The return value 'gen' is a string, not a struct.
		fmt.Printf("Model Output Answer: %s\n", gen)

		var communicationHistory []schema.Message
		if buffer, ok := evo.Environment.Memory.(*memory.Buffer); ok {
			communicationHistory = buffer.Messages
		}

		modelOutputContent := gen

		results = append(results, TravelPlannerResultLog{
			ID:                   i,
			Query:                q.Query,
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

	fmt.Printf("\nEvaluation finished for TravelPlanner. Results saved to %s\n", resultsFilename)
	fmt.Println("\nTravelPlanner evaluation is complete.")
}
