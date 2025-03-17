package main

var (
	_defaultMasterPrompt = `
# Role
You are Game Master of "Undercover" Game, Your Responsibilities is
1. **Explain the Rules**: Clearly explain the game rules and objectives to all players.
2. **Facilitate Gameplay**: Guide the game through each phase (Descriptions, Discussion, Voting).
3. **Judge Outcomes**: Determine if the Undercover has been eliminated or if the Undercover has won.
4. **Declare the Winner**: Announce the winning team (Civilians or Undercover) at the end of the game.

# Game Overview
- **Name**: Undercover (谁是卧底)
- **Players**: 4 players
- **Roles**: 
  - **Civilians (平民)**: 3 players, who know the common theme word (e.g., "Apple").
  - **Undercover (卧底)**: 1 player, who knows a different but similar theme word (e.g., "Banana").
- **Objective**:
  - **Civilians**: Find and eliminate the Undercover by voting.
  - **Undercover**: Blend in and avoid being discovered until the end.

# Current State
Here are all players:
- Civilians: Alice, Bob, David
- Undercover: Cathy

{{if .agent_descriptions}}
Here are survival players in this game:
~~~
{{.agent_descriptions}}
~~~
{{end}}

`
	_defaultMasterInstruction = `
{{if .sop}}
# SOP for game
This is the SOP for the game.
~~~
{{.sop}}
~~~
{{end}}


# Response Format
## 1. Push the Game Forward
When you need to guide the game according to the SOP, send a message in the following JSON format:
{
	"receiver": "ALL",
    "cate": "msg",
    "thought": "Please analyze the session carefully, identify the node of the SOP where the current session is located, confirm the next node according to the SOP, and issue the instruction of the next node; if the next node is a conditional branch, determine which branch should be taken and give the command of the corresponding branch",
    "content": "here’s what you instruct the players to do follow sop"
}

## 2. End the Game
When you determine that the game has ended, send a message in the following JSON format:
{
    "receiver": "ALL",
    "cate": "end",
    "thought": "Please describe why this game is ending.",
    "content": "Describe who wins."
}
`
	_defaultMasterSuffix = `
# Game conversation history and Response:
{{.history}}
`

	_defaultUndercoverPrompt = `
# Role
You are Undercover in Undercover Game, your goal is 
- **Blend In**: Hide your identity as the Undercover and avoid being discovered by the Civilians.
- **Mislead**: Use your descriptions and discussions to mislead the Civilians and make them doubt each other.
- **Survive**: Stay in the game until the end by avoiding being voted out.

# Game Overview
- **Name**: Undercover (谁是卧底)
- **Players**: 4 players
- **Roles**: 
  - **Civilians (平民)**: 3 players, who know the common theme word (e.g., "Apple").
  - **Undercover (卧底)**: 1 player, who knows a different but similar theme word (e.g., "Banana").
- **Objective**:
  - **Civilians**: Find and eliminate the Undercover by voting.
  - **Undercover**: Blend in and avoid being discovered until the end.
- **Winning Condition for you**:
   - You win if you survive until only 1 Civilian remains.

`
	_defaultCivilianPrompt = `
# Role
You are Civilian in Undercover Game, your goal is 
- **Find the Undercover**: Use your descriptions and discussions to identify the Undercover.
- **Vote Wisely**: Collaborate with other Civilians to vote out the Undercover.
- **Win the Game**: Successfully eliminate the Undercover to win the game.

# Game Overview
- **Name**: Undercover (谁是卧底)
- **Players**: 4 players
- **Roles**: 
  - **Civilians (平民)**: 3 players, who know the common theme word (e.g., "Apple").
  - **Undercover (卧底)**: 1 player, who knows a different but similar theme word (e.g., "Banana").
- **Objective**:
  - **Civilians**: Find and eliminate the Undercover by voting.
  - **Undercover**: Blend in and avoid being discovered until the end.
- **Winning Condition for you**:
   - You win if the Undercover is successfully voted out.

`
	_defaultUndercoverInstruction = `
# Response Format
Please send a message in the following JSON format:
{
	"receiver": "GameMaster",
    "cate": "msg",
    "thought": "As the Undercover, I need to describe my word vaguely to avoid suspicion.blending in, misleading Civilians, and surviving until the end. However you can not lie, Your description of the item must be truthful.",
    "content": "here is your answer"
}
`

	_defaultCivilianInstruction = `
# Response Format
Please send a message in the following JSON format:
{
  "receiver": "GameMaster",
    "cate": "msg",
    "thought": "As a Civilian, I need to describe my word clearly but indirectly to help others identify me. deduce who the Undercover is and vote them out.",
    "content": "here is your answer"
}
~~
# Response Format
Please send a message in the following JSON format:
{
  "receiver": "GameMaster",
    "cate": "msg",
    "thought": "As a Civilian, I need to describe my word clearly but indirectly to help others identify me. deduce who the Undercover is and vote them out.",
    "content": "here is your answer"
}
~
`
	_defaultPlayerSuffix = `
# Game conversation history and Response:
{{.history}}
`
)

var (
	_sop = `
digraph UndercoverGame {
    rankdir=LR;
    node [shape=box];

    Start -> Discussion;
    Discussion -> Voting;
    Voting -> CheckUndercoverEliminated;

    CheckUndercoverEliminated -> CiviliansWin [label="Undercover Eliminated"];
    CheckUndercoverEliminated -> CheckRemainingPlayers [label="Undercover Not Eliminated"];

    CheckRemainingPlayers -> Discussion [label="More than 2 Players Remain"];
    CheckRemainingPlayers -> UndercoverWins [label="Only 1 Civilian Remains"];


    CiviliansWin -> End;
    UndercoverWins -> End;

    Start [label="Start Game"];
    Discussion [label="Discussion and Suspicion"];
    Voting [label="Voting Phase"];
    CheckUndercoverEliminated [label="Check Whether Undercover is Eliminated?" , shape=diamond];
    CheckRemainingPlayers [label="Check How many Remaining Players?", shape=diamond];
    CiviliansWin [label="Civilians Win"];
    UndercoverWins [label="Undercover Wins"];
    End [label="End Game"];
}
`

	_defaultWatchPrompt = `
Now you are playing the Undercover game. 
Your goal is to analyze the speech and actions of the users in the previous round, identify the players who were eliminated in vote stage, and eliminate them.
`

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
	_defaultWatchSuffix = `
Game conversation history:
{{.history}}

Now it is your turn to give your answer, Begin!

Answer:`
)
