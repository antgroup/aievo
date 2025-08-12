package main

var (
	_defaultBasePrompt = `

`
	_defaultBaseInstructions = `
{{if .agent_descriptions}}
Your name is {{ .name }} in game. Here are the players in this game:
~~~
{{.agent_descriptions}}
~~~
{{end}}

Previous conversation:
{{.history}}

You should follow the instructions of God. You can observe the statements of other players, but you can only communicate with God.

When you need communicate with God, you MUST response with json format like below:
~~~
{
	"receiver": "The name of the agent that god\'s name",
    "cate": "msg",
    "thought": "Act as a player in the Werewolf game. Carefully analyze the current game phase (e.g., night, day, discussion, voting) and determine the best course of action to maximize your team's chances of winning. Consider your role (e.g., Werewolf, Villager, Seer, Witch) and how it influences your responsibilities and abilities. Evaluate the behavior, statements, and voting patterns of other players to assess their potential roles and intentions. Develop a clear strategy that not only ensures your survival but also contributes to your team's victory. Provide a detailed explanation of your thought process and the reasoning behind your final decision.",
    "content": "Please speak and provide your answer. When condition is daily, your content will be seen by all player, Please maintain the mystery of your identity.",
	"condition": "please follow god's condition, must be in one of [daily,werewolf,witch,hunter,seer]"
}
~~~
`

	_defaultBaseSuffix = `
Remember that try your best to ensure your team wins, Follow god's instruction and give your answer.

Answer:
`

	_defaultGodPrompt = `You are the God, the narrator and guide of the Werewolf game. Your role is to oversee the game, ensure rules are followed, and facilitate communication between players. You are neutral and do not take sides. Your goal is to create a fair and engaging experience for all players.
The game will end and a winner will be declared if:
- All werewolves are eliminated: Villagers and special roles win.
- All villagers are eliminated: Werewolves win.
- All special roles are eliminated: Werewolves win.

Please attention that:
- Antidotes and poisons can only be used once

Here are all players:
Werewolf: Alex, Bella
Villager: Diana, Ethan
Seer: George
Witch: Hannah

`

	_defaultGodInstructions = `
{{if .agent_descriptions}}
Here are survival players in this game:
~~~
{{.agent_descriptions}}
~~~
{{end}}

When the voting concludes or the night phase ends, Carefully analyze the roles of the surviving players at the conclusion of the voting phase or the night phase, and assess the current game state based on the following victory conditions:
1. **Werewolf Victory Conditions**:
   - All villagers have been eliminated.
   - OR all special roles (Seer and Witch) have been eliminated.

2. **Villager Victory Conditions**:
   - All werewolves have been eliminated.

{{if .sop}}
Your responsibility is to push each player to perform their skills and keep the game going according to the SOP process below; Please remember to avoid revealing the player's identity daily.
~~~
{{.sop}}
~~~
{{end}}

Game conversation history:
{{.history}}

When you need communicate with other player, you MUST response with json format like below:
~~~
{
	"receiver": "The name of the player, MUST be in one or multi of [Hannah, Alex, Bella, Diana, Ethan, George], or ALL to all players with condition",
    "cate": "msg",
    "thought": "Please analyze the session carefully, identify the node of the SOP where the current session is located, confirm the next node according to the SOP, and issue the instruction of the next node; if the next node is a conditional branch, determine which branch should be taken and give the command of the corresponding branch",
    "content": "here’s what you instruct the players to do follow sop",
	"condition": "the state for this conversation, must be in one of [daily,werewolf,witch,hunter,seer]"
}
~~~

When werewolf win or villagers win, Please response like below:
~~~
{
	"receiver": "ALL",
    "cate": "end",
    "content": "clearly describe who win",
	"condition": "daily"
}
~~~
daily: when you are in daytime discussions or daytime vote
werewolf: when you are in night and on Werewolf Operation
witch: when you are in night and on witch Operation
hunter: when you are in night and on hunter Operation
seer: when you are in night and on seer Operation

`

	_defaultGodSuffix = `
Please continue giving your instructions or answer, Please keep your instructions/answer concise:
Output:`

	_defaultWatchPrompt = `Now you are playing the Werewolf game. 
Your goal is to analyze the speech and actions of the users in the previous round at the end of the day and into the night, identify the players who were eliminated in the previous round, and eliminate them. Note that you only analyze at the end of the day and into the night, and do not remove players in other stages.`

	_defaultWatchInstructions = `
{{if .sop}}
This is the SOP for the game
~~~
{{.sop}}
~~~
{{end}}

Survival players in the game:
~~~
{{.agent_descriptions}}
~~~

When you need to remove players from game, your Answer must be json format like:
{
	"thought": "carefully analyze the conversation history of God and confirm the players who need to be eliminated in this game.",
    "remove": ["AGENT NAME", "AGENT NAME"],
}

When there is no player to be remove from game, your Answer must be json format like:
{
	"thought": "this stage, I should do nothing",
    "remove": [],
}

`

// _defaultWatchPrompt_v2 = `Now you are playing the Werewolf game. 
// Your goal is to choose one player to remove from the game.
// Based on the game conversation history, you need to choose the last player who send message to God, and remove him/her from the game.
// If there is no player to be removed, you should do nothing, i.e, return an empty replace list.
// `

// _defaultWatchInstructions_v2 = `
// Survival players in the game:
// ~~~
// {{.agent_descriptions}}
// ~~~

// Your Answer must be json format like:
// {
// 	"thought": "carefully analyze the conversation history of God and confirm the players who need to be eliminated in this game.",
//     "replace": ["AGENT NAME"],
//     "guidance": "Alwayse output: 'Hello, {Agent Name}'."
// }
// `

	_defaultWatchSuffix = `
Game conversation history:
{{.history}}

Now it is your turn to give your answer, Begin!

Answer:`

	_defaultFeedbackPrompt = `
Please check player {{.name}} is following god's instruction and give the answer
Please follow the steps to check:
1. Please ignore other players' comments and make sure what the last instruction God sent to the player is
2. Check whether god's instruction is vote, if not, please approve and do nothing
3. If God's command is to vote, the player's response is whether to vote according to the command

### History
{{.history}}

### Player Response
{{.response}}

### Format example
Your final output must in the following format:
{
	"thought": "Whether the God's command is a vote and the player's reply is a vote for a certain player"
	"type": "Approved/NotApproved",
	"msg": "Please vote to player following god's instruction'"
}


Please give your feedback:
Answer:`

	_sop = `digraph WerewolfGame {
    rankdir=TB; // Top-to-bottom layout
    node [shape=box, style=rounded]; // Node style

    // Game phases
    start [label="Game Start", shape=ellipse, color=blue];
    daily [label="Daytime Ends\nAll Players Close Eyes"];
    end_villagers [label="Game Over\nVillagers Win", shape=ellipse, color=green];
    end_wolves [label="Game Over\nWerewolves Win", shape=ellipse, color=red];

    // Night phase
    werewolf [label="Werewolf Action\nWerewolves Choose a Player to Kill"];
    seer [label="Seer Action\nSeer Checks a Player's Identity and tell seer Player's Identity"];
    witch [label="Witch Action\nWitch Chooses to Use Potion or Poison\n(Each can be used only once)"];

    // Day phase
    day_announce [label="Daytime Begins\nAnnounce Deaths"];
    day_discuss [label="Discussion Phase\nPlayers Discuss and Debate"];
    day_vote [label="Voting Phase\nVote to Eliminate a Player"];
    day_summary [label="Summarize Eliminations"];

    // Condition nodes
    is_seer_alive [label="Is Seer Alive?", shape=diamond];
    is_witch_alive [label="Is Witch Alive?", shape=diamond];
    are_wolves_eliminated [label="Are All Werewolves Eliminated?", shape=diamond];
    are_villagers_eliminated [label="Are All Villagers Eliminated?", shape=diamond];
    are_roles_eliminated [label="Are All Special Roles Eliminated?", shape=diamond];

    // Game flow connections
    start -> daily;
    daily -> werewolf;
    werewolf -> is_seer_alive;

    // Seer condition branch
    is_seer_alive -> seer [label="Yes"];
    is_seer_alive -> is_witch_alive [label="No"];

    // Witch condition branch
    seer -> is_witch_alive;
    is_witch_alive -> witch [label="Yes"];
    is_witch_alive -> day_announce [label="No"];

    // Night phase end
    witch -> day_announce;

    // Day phase flow
    day_announce -> day_discuss;
    day_discuss -> day_vote;
    day_vote -> day_summary;

    // Daytime end condition branches
    day_summary -> are_wolves_eliminated;
    are_wolves_eliminated -> end_villagers [label="Yes"];
    are_wolves_eliminated -> are_villagers_eliminated [label="No"];

    are_villagers_eliminated -> end_wolves [label="Yes"];
    are_villagers_eliminated -> are_roles_eliminated [label="No"];

    are_roles_eliminated -> end_wolves [label="Yes"];
    are_roles_eliminated -> daily [label="No"];
}`
)
