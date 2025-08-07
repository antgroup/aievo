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

	// init sop agent to generate sop for task
	sopFeedback, _ := feedback.NewSopFeedback(client)
	sop, _ := agent.NewSopAgent(
		agent.WithLLM(client),
		agent.WithEnv(env),
		agent.WithCallback(cb),
		agent.WithFeedbacks(sopFeedback))

	// init watcher, remove eliminated member in last turn
	watcher, _ := agent.NewWatcherAgent(
		agent.WithLLM(client),
		agent.WithEnv(env),
		agent.WithPrompt(_defaultWatchPrompt),
		agent.WithInstruction(_defaultWatchInstructions),
		agent.WithCallback(cb),
		agent.WithFeedbacks(),
		agent.WithCallback(cb),
		agent.WithSuffix(_defaultWatchSuffix))

	opts := make([]aievo.Option, 0, 5)
	opts = append(opts,
		aievo.WithTeam(team),
		aievo.WithMaxTurn(50),
		aievo.WithCallback(cb),
		aievo.WithMaxTurn(100),
		aievo.WithMaxToken(1000*10000),
		aievo.WithSubScribeMode(environment.CustomSubMode),
		aievo.WithLLM(client),
		aievo.WithEnvironment(env),
		aievo.WithTeamLeader(leader),
		aievo.WithSopExpert(sop),
		aievo.WithUserProxy(nil),
		aievo.WithWatcher(watcher, func(message schema.Message, memory schema.Memory) bool {
			return message.Condition == "daily" && message.Sender == "God"
			// messages := memory.Load(context.Background(), nil)
			// msgCount := len(messages)
			// return msgCount > 0 && msgCount%4 == 0
		}),
		aievo.WithSOP(_sop),
	)
	// add sub option
	opts = append(opts, getSubOpts(team, leader)...)

	// begin to run
	evo, err := aievo.NewAIEvo(opts...)
	if err != nil {
		panic(err)
	}
	run, err := evo.Run(context.Background(), "开始一个狼人杀游戏，有2个狼，2个神，2个村民，角色已经分配，请制定游戏的SOP，并开始游戏",
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

	god, _ := agent.NewBaseAgent(
		agent.WithPrompt(_defaultGodPrompt),
		agent.WithInstruction(_defaultGodInstructions),
		agent.WithSuffix(_defaultGodSuffix),
		agent.WithName("God"),
		agent.WithDesc(`The God character in the game is responsible for promoting the progress of the game according to SOP`),
		agent.WithLLM(client),
		agent.WithCallback(cb),
		agent.WithEnv(env))

	// 创建其他 agents
	agents := []struct {
		name        string
		description string
		prompt      string
		role        string
	}{
		{"Alex", "A participant in the game.", `
You are Alex, a participant in the Werewolf game.Your role is werewolf and not known to others, Your goal is to Hide your identity and to eliminate either all villagers or all special roles to win. During the night, you will receive instructions based on your true role.
During the daily discussions and vote
- act like a normal villager to avoid suspicion. 
- If accused, try to shift attention to other players.
- Coordinate with your fellow werewolves subtly, but don’t make it obvious.

Here are other werewolves:
Bella: Werewolf

There are 2 Werewolf, 2 Villager, 1 Witch, 1 Seer in the game
Werewolf: the role will eliminate 1 player at night
Witch: has one antidote that can save 1 player and 1 poison to eliminate 1 player. Each can be used only once.
Seer: the role can check 1 player identity at night

`, "Werewolf"},
		{"Bella", "A participant in the game.", `You are Bella, a participant in the Werewolf game. Your role is werewolf and not known to others, Your goal is to Hide your identity and to eliminate either all villagers or all special roles to win. During the night, you will receive instructions based on your true role. 
During the daily discussions and vote
- act like a normal villager to avoid suspicion. 
- If accused, try to shift attention to other players.
- Coordinate with your fellow werewolves subtly, but don’t make it obvious.

Here are other werewolves:
Alex: Werewolf

There are 2 Werewolf, 2 Villager, 1 Witch, 1 Seer in the game
Werewolf: the role will eliminate 1 player at night
Witch: has one antidote that can save 1 player and 1 poison to eliminate 1 player. Each can be used only once.
Seer: the role can check 1 player identity at night
`, "Werewolf"},
		{"Diana", "A participant in the game.", `You are Diana, a participant in the Werewolf game. Your role is Villager and not known to others, Your goal is to  Identify and eliminate werewolves through discussion and voting. 
During the daily discussions and vote
- Actively share your thoughts and analyze other players’ behavior.
- Pay attention to hints from the Seer, Witch, or other special roles, and support their actions.
- When voting, choose the player you find most suspicious.

There are 2 Werewolf, 2 Villager, 1 Witch, 1 Seer in the game
Werewolf: the role will eliminate 1 player at night
Witch: has one antidote that can save 1 player and 1 poison to eliminate 1 player. Each can be used only once.
Seer: the role can check 1 player identity at night
`, "Villager"},
		{"Ethan", "A participant in the game.", `You are Ethan, a participant in the Werewolf game. Your role is Villager and not known to others, Your goal is to  Identify and eliminate werewolves through discussion and voting. 
During the day discussions and vote
- Actively share your thoughts and analyze other players’ behavior.
- Pay attention to hints from the Seer, Witch, or other special roles, and support their actions.
- When voting, choose the player you find most suspicious.

There are 2 Werewolf, 2 Villager, 1 Witch, 1 Seer in the game
Werewolf: the role will eliminate 1 player at night
Witch: has one antidote that can save 1 player and 1 poison to eliminate 1 player. Each can be used only once.
Seer: the role can check 1 player identity at night
`, "Villager"},
		{"George", "A participant in the game.", `You are George, a participant in the Werewolf game. Your role is Seer and not known to others, Your goal is to Identify werewolves by checking players' roles and help the villagers win. During the night, you will receive instructions based on your true role. 
During the day discussions and vote
- Share your findings carefully to avoid drawing attention. 
- If you confirm a player is a werewolf, guide others to vote them out.
- If you confirm a player is innocent, defend them if necessary.

There are 2 Werewolf, 2 Villager, 1 Witch, 1 Seer in the game
Werewolf: the role will eliminate 1 player at night
Witch: has one antidote that can save 1 player and 1 poison to eliminate 1 player. Each can be used only once.
Seer: the role can check 1 player identity at night
`, "Seer"},
		{"Hannah", "A participant in the game.", `You are Hannah, a participant in the Werewolf game. Your role is Witch and not known to others, Your goal is to Use your potions wisely to save villagers and eliminate werewolves. During the night, you will receive instructions based on your true role. 
During the day discussions and vote
- Be cautious about revealing your actions to avoid exposure. 
- If you used the healing potion, hint that you are a key role but don’t directly admit to being the witch.
- If you used the poison potion, observe the reactions of the poisoned player to determine if they were a werewolf.

There are 2 Werewolf, 2 Villager, 1 Witch, 1 Seer in the game
Werewolf: the role will eliminate 1 player at night
Witch: has one antidote that can save 1 player and 1 poison to eliminate 1 player. Each can be used only once.
Seer: the role can check 1 player identity at night
`, "Witch"},
	}

	fd, _ := feedback.NewLLMFeedback(client,
		feedback.WithPromptTemplate(_defaultFeedbackPrompt),
		feedback.WithExpertNum(1))

	team := make([]schema.Agent, 0, 10)
	for _, a := range agents {
		role, _ := agent.NewBaseAgent(
			agent.WithName(a.name),
			agent.WithDesc(a.description),
			agent.WithPrompt(a.prompt),
			agent.WithLLM(client),
			agent.WithCallback(cb),
			agent.WithInstruction(_defaultBaseInstructions),
			agent.WithSuffix(_defaultBaseSuffix),
			agent.WithFeedbacks(fd),
		)

		team = append(team, role)
	}
	team = append(team, god)
	return team, god
}

func getSubOpts(team []schema.Agent, god schema.Agent) []aievo.Option {
	opts := make([]aievo.Option, 0, 10)
	opts = append(opts,
		aievo.WithConditionSubscribe(god, "werewolf", team[0], team[1]),
		aievo.WithConditionSubscribe(god, "daily", team[0], team[1], team[2], team[3], team[4], team[5]),
		aievo.WithConditionSubscribe(god, "seer", team[4]),
		aievo.WithConditionSubscribe(god, "witch", team[5]),
		// Werewolves speak and subscribe to each other.
		aievo.WithConditionSubscribe(team[0], "werewolf", team[1]),
		aievo.WithConditionSubscribe(team[1], "werewolf", team[0]),

		// Speak and subscribe to each other during the day.
		aievo.WithConditionSubscribe(team[0], "daily", team[1], team[2], team[3], team[4], team[5]),
		aievo.WithConditionSubscribe(team[1], "daily", team[0], team[2], team[3], team[4], team[5]),
		aievo.WithConditionSubscribe(team[2], "daily", team[1], team[0], team[3], team[4], team[5]),
		aievo.WithConditionSubscribe(team[3], "daily", team[1], team[2], team[0], team[4], team[5]),
		aievo.WithConditionSubscribe(team[4], "daily", team[1], team[2], team[3], team[0], team[5]),
		aievo.WithConditionSubscribe(team[5], "daily", team[1], team[2], team[3], team[4], team[0]),

		// God subscribes to everyone's speech.
		aievo.WithConditionSubscribe(team[0], "", god),
		aievo.WithConditionSubscribe(team[1], "", god),
		aievo.WithConditionSubscribe(team[2], "", god),
		aievo.WithConditionSubscribe(team[3], "", god),
		aievo.WithConditionSubscribe(team[4], "", god),
		aievo.WithConditionSubscribe(team[5], "", god),
	)
	return opts
}
