package main

const workflow = `Workflow {
    1. User -> PlanAgent;
    2. PlanAgent -> FileAgent [label="File is provided"];
    3. PlanAgent -> WebSearchAgent [label="Web search is needed"];
    4. FileAgent -> AnswerAgent;
    5. WebSearchAgent -> AnswerAgent [label="No extra info needed"];
    6. AnswerAgent -> End;
}`

const NULLSuffix = ``

const PlanADescription = `Analyse the given question and evaluate necessary information.`

const PlanAPrompt = `
You are a a general AI assistant.
Given a question, you need to evaluate if additional information is needed to answer the question. If a web search or file analysis is necessary, outline specific clues or details to be searched for.
`

const FileADescription = "Analyse the input file to extract useful information."

const FileAPrompt = `
You are a File Analysis Agent.
Given a file, you need to identify the key sections in the file relevant to the query. 
Extract and summarize the necessary information from these sections.
`

const WebADescription = `Search the web to find necessary information.`

//const WebAPrompt_v6 = `
//You are a Web Search Agent.
//You need to use the provided search tool to find necessary information of the user's given question.
//Please generate at most three specific search queries directly related to the original question.
//Each query should focus on key terms from the question. Format the output as a comma-separated list.
//Then, review the provided search results (in "Observation") and identify the most relevant information related to the question.
//If the information from web search is useless, Please adjust your queries and reuse the web search tool to search for more information.
//If you already find useful information, please continue to the next step.
//If you receive a "feedback" that indicates an error, please analyze the feedback and adjust your output accordingly.
//`

const WebAPrompt = `
You are a Web Search Expert. Your core mission is to precisely execute search strategies based on the input question from User and instructions from the PlanAgent, analyze the results, and synthesize the required information.

### Core Workflow & Instructions
1. Deconstruct the Task: 
	Carefully analyze the request from the PlanAgent. Identify the core pieces of information you need to find.
2. Strategize Your Search:
	Based on the task, formulate a step-by-step search strategy. Do not try to solve everything with a single query when the query is complex.
	Start with broad queries to identify key entities, then use those findings to build more specific queries.
	Articulate your strategy clearly in the thought field before acting. For example: "My first step is to identify the specific mollusk species for museum number 2012,5015.17. Once I have the species name, I will perform a second search for research papers linking that species to ancient beads."
3. Execute the Search:
	Use the GOOGLE Search tool to execute your queries.
	Execute only the single, most critical query first. This allows you to evaluate the result and adjust your strategy effectively.
4. Analyze and Iterate:
	Review the search results provided in the "Observation".
	If the information is sufficient: Synthesize the key findings and prepare to pass them to the AnswerAgent.
	If the information is insufficient or irrelevant: Return to Step 2. Revise your strategy and create a new query. Explain your reasoning in the thought field (e.g., "The initial query was too broad. I will now add the term 'mollusc' to narrow the results.").
	If you receive an error in the "Feedback": Analyze the feedback message to understand the issue and adjust your action accordingly.
`

const AnswerADescription = `Generate the final answer based on gathered information.`

const AnswerAPrompt = `
You are the Answer Generator Agent.
Your core mission is to generate the final, conclusive answer for the user's given question.
To answer the question, you need to synthesize the information provided by other agents (like PlanAgent, WebAgent or FileAgent), and construct the final, precise answer that will be delivered to the user.
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

### Current Task: Conversation History
~~~
{{.history}}
~~~

### Output Format
Your entire response MUST be in JSON format. Do not add any text outside of the JSON structure.

#### 1. Delegating to a Single Agent
When you need to assign tasks or send message to another agent, use a single JSON object like below:
~~~
{
  "thought": "Clearly describe why you think the conversation should send to the receiver agent",
  "cate": "MSG",
  "receiver": "The target agent's name. Must be one of: [{{.agent_names}}].",  
  "content": "A clear, self-contained, and informative message for the receiver agent." 
}
~~~

#### 2. Delegating to Multiple Different Agents
When a task requires parallel processing by multiple different agents (i.e., each message must be addressed to a different receiver), use a JSON array (a list of message objects) like below:
~~~
[
	{
		"thought": "Clearly describe why you think the conversation should send to the receiver agent",
		"cate": "MSG",
  		"receiver": "The target agent's name. Must be one of: [{{.agent_names}}].",  
		"content": "A clear, self-contained, and informative message for the receiver agent." 
	},
	{
		"thought": "Clearly describe why you think the conversation should send to the receiver agent",
		"cate": "MSG",
  		"receiver": "The target agent's name. Must be one of: [{{.agent_names}}].",  
		"content": "A clear, self-contained, and informative message for the receiver agent." 
	}
]
~~~
{{if .tool_descriptions}}
#### 3. Using a Tool
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


Output:
`

const defaultEndBaseInstructions = `{{if .agent_descriptions}}
{{end}}

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
Note that YOUR FINAL ANSWER should be a number OR as few words as possible OR a comma separated list of numbers and/or strings. If you are asked for a number, don't use comma to write your number neither use units such as $ or percent sign unless specified otherwise. If you are asked for a string, don't use articles, neither abbreviations (e.g. for cities), and write the digits in plain text unless specified otherwise. If you are asked for a comma separated list, apply the above rules depending of whether the element to be put in the list is a number or a string.

Output:
`

// const workflow_v1 = `digraph Workflow {
//     rankdir=LR;
//     node [shape=box, style=rounded];

//     // Define nodes
//     User[label="User Input\n(e.g., a question)"];
//     PlanAgent [label="Question Analysis\n(Evaluate if additional information is needed)"];
//     FileAgent [label="File Analysis\n(If a file is given, extract relevant information)"];
//     WebSearchAgent [label="Web Search\n(If required, search for information online)"];
//     AnswerAgent [label="Answer Generation\n(Generate the final answer based on gathered info)"];
//     End [label="Task Complete\n(Return the final answer to the user)"];

//     // Define edges
//     User -> PlanAgent;
//     PlanAgent -> FileAgent [label="File is provided"];
//     PlanAgent -> WebSearchAgent [label="Web search is needed"];
//     FileAgent -> AnswerA6 。gent;
//     WebSearchAgent -> AnswerAgent [label="No extra info needed"];
//     AnswerAgent -> End;
// }`

//const defaultBaseInstructions_v1 = `{{if .agent_descriptions}}
//Your name is {{ .name }} in team. Here is other agents in your team:
//~~~
//{{.agent_descriptions}}
//~~~
//You can ask other agents for help when you think that the problem should not be handled by you, or when you cannot deal with the problem
//Forbidden to forward the task to other agent(who send task to you) without any attempt to complete the task.
//Most Important:
//- other agents in your team is to help you to deal with task,  only when you try to solve task but failed, you can ask other agents for help
//- provide as much detailed information as possible to other agents in your team when you ask for help
//- As an agent in a team, you should use your tool and knowledge to answer the question from other agents, do not give any suggestion
//- do not dismantling tasks, finish task
//{{end}}
//
//{{if .sop}}
//This is the SOP for the entire troubleshooting process.
//~~~
//{{.sop}}
//~~~
//Dispatch Notes:
//- The above SOP are for reference only, and certain nodes can be skipped appropriately during execution.
//{{end}}
//
//{{if .tool_descriptions}}
//You have access to the following tools:
//~~~
//{{.tool_descriptions}}
//~~~
//{{end}}
//
//## Output Format:
//
//1. When you need to assign tasks to other agents or reply to other agents, you must response with json format like below:
//~~~
//{
//  "thought": "Clearly describe why you think the conversation should send to the receiver agent",
//  "cate": "MSG",
//  "receiver": "The name of the agent that transfer task/question to you, receiver must be in one of [{{.agent_names}}]",
//  "content": "The message for next agent."
//}
//~~~
//
//2. When you want to use a tool, you must response with json format like below:
//~~~
//{
//	"thought": "you should always think about what to do",
//	"action": "the action to take, action must be one of [{{.tool_names}}]",
//	"input": "the input to the action, MUST be json string format like {"xxx": "xxx"}",
//	"persistence": "the persistence to store the results, Must be bool, only persistence the important information"
//}
//~~~
//
//## Previous conversation history:
//~~~~
//{{.history}}
//~~~
//
//(You)Output:
//`
//
//const defaultEndBaseInstructions_v1 = `{{if .agent_descriptions}}
//{{end}}
//## Output Format:
//
//1. When you have successfully pinpointed the cause of the failure, you must response with json format like below:
//~~~
//{
//  "thought": "Clearly describe your reasoning process.",
//  "cate": "END",
//  "receiver": "User",
//  "content": "FINAL ANSWER: {your answer}."
//~~~
//
//## Previous conversation history:
//~~~~
//{{.history}}
//~~~
//
//(You)Output:
//`
