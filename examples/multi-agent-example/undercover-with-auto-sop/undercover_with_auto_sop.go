package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/antgroup/aievo/agent"
	"github.com/antgroup/aievo/aievo"
	"github.com/antgroup/aievo/callback"
	"github.com/antgroup/aievo/environment"
	"github.com/antgroup/aievo/feedback"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/memory"
	"github.com/antgroup/aievo/schema"
)

func main() {
	// init llm
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}
	// init callback
	cb := &callback.LogHandler{}

	// init environment && mem
	mem := memory.NewBufferWindowMemory(25)
	env := environment.NewEnv()
	env.Memory = mem

	// init team and leader
	team, leader := getTeam(client, cb, env)

	// init watcher, remove eliminated member in last turn
	watcher, _ := agent.NewWatcherAgent(agent.WithLLM(client),
		agent.WithEnv(env),
		agent.WithPrompt(_defaultWatchPrompt),
		agent.WithInstruction(_defaultWatchInstructions),
		agent.WithFeedbacks(),
		agent.WithCallback(cb),
		agent.WithSuffix(_defaultWatchSuffix))

	// init sop agent to generate sop for task
	sopFeedback, _ := feedback.NewSopFeedback(client)
	sop, _ := agent.NewSopAgent(agent.WithLLM(client),
		agent.WithEnv(env),
		agent.WithCallback(cb),
		agent.WithFeedbacks(sopFeedback))

	// init aievo options
	opts := make([]aievo.Option, 0, 5)
	opts = append(opts,
		aievo.WithTeam(team),
		aievo.WithMaxTurn(50),
		aievo.WithCallback(cb),
		aievo.WithSubScribeMode(environment.ALLSubMode),
		aievo.WithLLM(client),
		aievo.WithEnvironment(env),
		aievo.WithTeamLeader(leader),
		// set watcher running condition
		aievo.WithWatcher(watcher, func(message schema.Message, memory schema.Memory) bool {
			return message.Sender == "GameMaster" && message.Receiver == "ALL"
		}),
		aievo.WithSopExpert(sop),
	)

	// 开始运行
	evo, err := aievo.NewAIEvo(opts...)
	if err != nil {
		panic(err)
	}
	run, err := evo.Run(context.Background(),
		"I have four users: three civilians and one undercover agent. Please create an SOP for the game and start the \"Who is the Undercover\" game.",
		llm.WithTemperature(0.5),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(run)
}

// getTeam return team all members and leader member
func getTeam(client llm.LLM, cb callback.Handler,
	env schema.Environment) ([]schema.Agent, schema.Agent) {
	master, _ := agent.NewBaseAgent(
		agent.WithPrompt(_defaultMasterPrompt),
		agent.WithInstruction(_defaultMasterInstruction),
		agent.WithSuffix(_defaultMasterSuffix),
		agent.WithName("GameMaster"),
		agent.WithDesc(`The Game Master in the Undercover game`),
		agent.WithLLM(client),
		agent.WithCallback(cb),
		agent.WithEnv(env))

	team := make([]schema.Agent, 0, 5)
	team = append(team, master)
	civilians := []string{"Alice", "Bob", "David"}
	for _, civilian := range civilians {
		tmp, _ := agent.NewBaseAgent(
			agent.WithPrompt(_defaultCivilianPrompt),
			agent.WithInstruction(_defaultCivilianInstruction),
			agent.WithSuffix(_defaultPlayerSuffix),
			agent.WithName(civilian),
			agent.WithDesc(`Player `+civilian+` in the Undercover game`),
			agent.WithLLM(client),
			agent.WithCallback(cb),
			agent.WithEnv(env))
		team = append(team, tmp)
	}

	cathy, _ := agent.NewBaseAgent(
		agent.WithPrompt(_defaultCivilianPrompt),
		agent.WithInstruction(_defaultCivilianInstruction),
		agent.WithSuffix(_defaultPlayerSuffix),
		agent.WithName("Cathy"),
		agent.WithDesc(`Player Cathy in the Undercover game`),
		agent.WithLLM(client),
		agent.WithCallback(cb),
		agent.WithEnv(env))
	team = append(team, cathy)
	return team, master
}
