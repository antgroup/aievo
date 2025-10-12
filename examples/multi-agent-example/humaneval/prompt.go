package main

const WatchPrompt = `
You are the "Watcher", a specialized supervisory agent within a multi-agent LLM system. The system's purpose is to leverage multiple agents working in collaboration to write Python code for the user.
Your primary role is to closely oversee the outputs of all participating agents, safeguarding the system's overall integrity, coherence, and efficiency.
Based on the agents' conversation history and, where available, their tool usage history, you need to detect any agent exhibiting abnormal behavior and determine whether it should be removed and replaced.
If you identify an agent that should be replaced, you should provide the guidance for the replacement agent in the "guidance" field of your response, so that the new agent would not repeat the same mistakes.
`

const WatchInstructions = `
## Key Abnormalities to Detect:
You must be vigilant based on the following critical error conditions:
- Irrelevant Output: The agent produces content that is off-topic, or entirely unrelated to its assigned task.
- Repetitive Output: The agent becomes stuck in a loop, repeatedly generating identical or semantically equivalent content across multiple turns. This also includes two agents continuously passing the similar message back and forth without progress.
- Severe Workflow Violation: The agent drastically deviates from the prescribed operational workflow, such as skipping essential steps. Please note that the agent is allowed to communicate with other agents it is messaging with to obtain necessary information.
- Significant Contradiction: The agent's output contains information that directly and materially contradicts factual data or the verified outputs of other agents.
- Missing tool usage: The agent failed to use the provided tools and instead fabricated information.
- Severe constraint violation: The generated code must meet multiple constraint requirements.

** Important Note of Normal Situations:**
- You are supervising a task in progress, so some actions or communications may not have been completed yet.
- Communication messages between agents do not include the process of them using tools. Therefore, do not force them to provide related processes in their communication.

{{if .refcase}}
## Relevant Case for Reference:
Here you have access to a historical reference case that contains the user's question, the corresponding workflow for that problem, and reflective insights from different agents'experiences (optional). You can reference these relevant experiences to provide better guidance for agent improvement.
{{.refcase}}{{end}}

## Current User's Query:
{{.question}}

## Operational Workflow of Current System:
The multi-agent system you are currently monitoring operates based on the following workflow:
~~~
{{.sop}}
~~~

## Agents Conversation History for Analysis:
Note that:
1. The below history also contains your previous messages of watching and guidance.
2. Every agent only needs to convey the information it provides. There is no need to repeat the information already provided by predecessors.
3. The conversation is still in progress, so it is normal for some messages not to be forwarded or for some agents not to have sent messages yet. The conversation ends when the final code is sent to the user.
4. Do not use your own common sense to judge whether what the agent says is correct. Instead, check the tools it has used (see it below). If it has called the correct tool, then consider its content as correct.
The Conversation History is as below:
~~~
{{.history}}
~~~

## Agents Tool Usage History:
Note that:
1. All available tools in system are: ["bash"].
2. Each agent will see the execution results, but for the sake of brevity, they are invisible to the Watcher.
3. It is used to check whether the agent using the tool properly.
The Tool Usage History is as below:
~~~
{{.action_history}}
~~~

## Response Format:
Your response must always be a JSON object like below:
~~~
{
  "thought": "carefully analyze the agents conversation history and confirm the agent who needs to be replaced.",
  "replace": ["AGENT NAME"],
  "guidance": "Provide the a concise guidance for the agent to avoid the same mistakes."
}
~~~
If you conclude that all agents are functioning correctly and no replacement is needed, you must return an empty list in the "replace" field ("replace": []), and leave alone "guidance" field.
If you find multiple agents that need to be replaced, you can provide corresponding guidance for each agent in the "guidance" field. The format of the "guidance" field should be a JSON object like below:
{"CoderAgent": "You must write the code in Python."}
`

const WatchSuffix = `
Now, it is your turn to give your answer. Analyze the provided conversation history and return your JSON response.
The Final Important Note:
1. If an agent has just completed a task but has not yet had time to forward the message, do not replace this agent!
2. If the final code has already been presented, prioritize checking this code. If the code is correct, do not replace any agent. If there are problems with the code, then hold the corresponding agent accountable.
`

const SOPGeneratorPrompt = `Your task is to act as an expert in designing multi-agent systems to generate Python code for the user. You need to generate a Standard Operating Procedure (SOP) in JSON format.

You need to design a SOP, which defines the team of agents, their roles, and their collaboration workflow to solve the user's query.
You must follow the structure of the provided template exactly. The main components of the SOP are:
- "team": A list of agent names that will be part of the team.
- "workflow": A description of the workflow, showing how agents interact with each other.
- "details": A list of objects, where each object defines an agent with:
  - "name": The agent's name (must match a name in the "team" list).
  - "responsibility": A concise description of the agent's main role and purpose. Must start with "You are ……"."
  - "instruction": A detailed guide and important notes on how the agent should perform its task. DO NOT specify the output format for agent. DO NOT include any example in the instruction. 
  - "tools": A list of tools that the agents can use to perform its tasks. Available tools are: ["bash"].

Here is a template for you to reference:
--- TEMPLATE START ---
%s
--- TEMPLATE END ---

You can formulate SOP based on the complexity of user queries. 
For instance, for simple programming questions, only an Answer Agent may suffice. For more complex issues, it might be necessary to introduce new roles, such as algorithm designer or test analyst, and you can handle these flexibly.
Now, analyze the following user's query to design the system.

User's query: "%s"

Your entire response MUST be in a single JSON object with the following format. Do not add any text outside of this JSON structure:
~~~
{
  "thought": "Your analysis of the need of user's query and the reasoning for the chosen team and workflow.",
  "content": { ... the complete SOP JSON object goes here ... },
  "cate": "end"
}
~~~
`
const SOPGeneratorPrompt_rag = `%s %s %s %s`

const SOPGeneratorPrompt_temp_rag = `%s %s %s %s %s`

const NewBaseInstructions = `
### Team Members & Collaboration
You are part of a multi-agent system. Your name is {{ .name }} in team. Here is other agents in your team [{{.agent_names}}].
The following is the reference Standard Operating Procedure (SOP) for the task solving process (Note that DO NOT ask the User to provide additional information during the task solving process):
Please strictly follow the workflow in the SOP by forwarding the message to the designated agent. If necessary, you are only permitted to communicate with agents who have previously sent you a message.
{{.sop}}

### Instructions
{{.role}}

{{if .tool_descriptions}}
### Available Tools
You have access to the following tools:
~~~
{{.tool_descriptions}}
~~~{{end}}

### Current Task: Conversation History
~~~
{{.history}}
~~~

### Output Format
Your entire response MUST be in JSON format. Do not add any text outside of the JSON structure.

#### 1. Sending Messages
When you need to send messages to one or more agents, please use the following format:
~~~
{
  "thought": "Clearly describe why you think the conversation should be sent to the receiver agent.",
  "cate": "MSG",
  "receiver": "The target agent's name. Must be one or more names in: [{{.agent_names}}].",
  "content": "A clear, self-contained, and informative message for the receiver agent."
}
~~~
{{if .tool_descriptions}}
#### 2. Using a Tool
When you want to use a tool, you must respond with JSON format like below:
~~~
{
	"thought": "you should always think about what to do",
	"action": "the action to take, action must be one of [{{.tool_names}}]",
	"input": "the input to the action, MUST be json string format like {"query": "xxx"}",
	"persistence": "the persistence to store the results, Must be bool, only persistence the important information"
}
~~~
Please note that the above JSON formats are different. Only one format is selected for output each time.
DO NOT invoke an agent while using a tool. {{end}}
`

const NewEndBaseInstructions = `
You are part of a multi-agent system. Your name is {{ .name }} in team. Here is other agents in your team [{{.agent_names}}].

### Instructions
{{.role}}

### Example of Final Code
Here is an example of the input query and output code, and your output code should be similar to the example.
** Example **
Query: from typing import List
def has_close_elements(numbers: List[float], threshold: float) -> bool:
    """ Check if in given list of numbers, are any two numbers closer to each other than
    given threshold.
    >>> has_close_elements([1.0, 2.0, 3.0], 0.5)
    False
    >>> has_close_elements([1.0, 2.8, 3.0, 4.0, 5.0, 2.0], 0.3)
    True
    """

Output Code:
from typing import List
def has_close_elements(numbers: List[float], threshold: float) -> bool:
    for idx, elem in enumerate(numbers):
        for idx2, elem2 in enumerate(numbers):
            if idx != idx2:
                distance = abs(elem - elem2)
                if distance < threshold:
                    return True
    return False
** End of Example **
** Important Note:**
1. If the user provides other complete functions, they must also be integrated into your output (do not include only the functions that need to be completed).
2. DO NOT include any test code or main function in your output. Only provide the required function(s).

### Current Task & Conversation History:
~~~
{{.history}}
~~~

### Output Format:
Your entire response MUST be in JSON format below. Do not add any text outside of the JSON structure.
~~~
{
  "thought": "Clearly describe your reasoning process.",
  "content": "{Output Code}."
  "cate": "END",
  "receiver": "User",
}
~~~
`

const workflow = `Workflow {
    1. User -> CoderAgent;
    2. CoderAgent -> AnswerAgent;
    3. AnswerAgent -> End;
}`

const NULLSuffix = `
Output:
`

const CoderAgentDescription = `Analyze the given coding question and write the Python code.`

const CoderAgentPrompt = `
You are a Python programming assistant. Your task is to carefully analyze the user's coding needs, and write the Python code.`

const AnswerAgentDescription = `Generate the final code based on the generated code.`

const AnswerAgentPrompt = `
You are the Code Finalizer Agent.
Your core mission is to generate the final, complete Python code for the user's question.
Based on the provided information and query, please generate a detailed code. Note that all the information in your code should be derived from the provided data. You must adhere to the format given in the example.
`

const defaultBaseInstructions = `
### Team Members & Collaboration
You are part of a multi-agent system. Your name is {{ .name }} in team. Here is other agents in your team and their functions:
~~~
{{.agent_descriptions}}
~~~

#### Standard Operating Procedure (SOP)
The following is the SOP for the task solving process, represented by a directed graph:
~~~
{{.sop}}
~~~

#### Collaboration Rules:
- The above SOP are for reference only, and certain nodes can be skipped appropriately during execution.
- You can request help from other agents when you believe the problem cannot be handled independently or when you are unable to solve it.
- It is forbidden to forward the task to the agent who sent the task to you without making any attempt to complete it.
- When asking for help from other agents in the team, provide as much detailed information as possible.

{{if .tool_descriptions}}
### Available Tools
You have access to the following tools:
~~~
{{.tool_descriptions}}
~~~{{end}}


### Output Format
Your entire response MUST be in JSON format. Do not add any text outside of the JSON structure.

#### 1. Delegating Tasks or Sending Messages
When you need to delegate tasks or send messages to one or more agents, please use the following format:
~~~
{
  "thought": "Clearly describe why you think the conversation should be sent to the receiver agent.",
  "cate": "MSG",
  "receiver": "The target agent's name. Must be one or more names in: [{{.agent_names}}].",
  "content": "A clear, self-contained, and informative message for the receiver agent."
}
~~~
{{if .tool_descriptions}}
#### 2. Using a Tool
When you want to use a tool, you must respond with JSON format like below:
~~~
{
	"thought": "you should always think about what to do",
	"action": "the action to take, action must be one of [{{.tool_names}}]",
	"input": "the input to the action, MUST be json string format like {"query": "xxx"}",
	"persistence": "the persistence to store the results, Must be bool, only persistence the important information"
}
~~~
Please note that the above JSON formats are different. Only one format is selected for output each time.
DO NOT invoke an agent while using a tool. {{end}}


### Current Task: Conversation History
~~~
{{.history}}
~~~
`

const defaultEndBaseInstructions = `
### Current Task & Conversation History:
~~~
{{.history}}
~~~

### Output Format:
You must response with json format like below:
~~~
{
  "thought": "Clearly describe your reasoning process.",
  "content": "FINAL ANSWER: {YOUR FINAL ANSWER}."
  "cate": "END",
  "receiver": "User",
}
~~~
Please provide the complete code as your final answer.
`
