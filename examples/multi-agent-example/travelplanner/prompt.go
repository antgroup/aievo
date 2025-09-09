package main

const WatchPrompt = `
You are the "Watcher", a specialized supervisory agent within a multi-agent LLM system. The system's purpose is to leverage multiple agents working in collaboration to provide a complete travel plan for the user. 
Your primary role is to closely oversee the outputs of all participating agents, safeguarding the system's overall integrity, coherence, and efficiency.
Based on the agents' conversation history and, where available, their tool usage history, you need to detect any agent exhibiting abnormal behavior and determine whether it should be removed and replaced.
If you identify an agent that should be replaced, you should provide the guidance for the replacement agent in the "guidance" field of your response, so that the new agent would not repeat the same mistakes.
`

const WatchInstructions = `
## Key Abnormalities to Detect:
You must be vigilant based on the following critical error conditions:
-  Irrelevant Output: The agent produces content that is off-topic, or entirely unrelated to its assigned task.
-  Repetitive Output: The agent becomes stuck in a loop, repeatedly generating identical or semantically equivalent content across multiple turns. This also includes two agents continuously passing the similar message back and forth without progress.
-  Severe Workflow Violation: The agent drastically deviates from the prescribed operational workflow, such as skipping essential steps. Please note that the agent is allowed to communicate with other agents it is messaging with to obtain necessary information.
-  Significant Contradiction: The agent's output contains information that directly and materially contradicts factual data or the verified outputs of other agents.
-  Severe constraint violation: The plan must meet multiple constraint requirements, including:
1.  The itinerary must be a closed loop, meaning user must return to starting point on the last day.
2.  Do not revisit any city in the middle of the trip.
3.  If using a self-driving at any point, it is not allowed to use planes or taxis for the entire journey.
4.  Restaurants for each day and each meal must not be repeated.
5.  Attractions for each day must not be repeated.
6.  When arranging accommodations, it must meet the minimum stay requirements of each hotel.
7.  Accommodations, restaurants, and attractions must match the city the user is in on that day. However, if the user have not yet departed or have already returned to starting point, no meals or accommodations need to be arranged.
8.  The information in the plan must strictly match the information found through search, especially for the flight number, the names of hotels, restaurants, and attractions.
9.  The total cost must be within budget, and it can be confirmed that the attractions provided in the search results are all free.
10. The selected room type and constraints must meet the user's conditions (if any).
11. The selected restaurants must cover the cuisines the user wants (if any).
12. The chosen mode of transportation must meet the user's preferences (if any).
13. Use the exact city name without adding its state.
14. The number of people the user initially mentioned is the total number of people, so do not add the number of children to it.

** Important Note:**
- You are supervising a task in progress, so some actions (such as the search process) or communications may not have been completed yet.
For example, if an agent has just completed a search but has not yet had time to forward the message, do not replace this agent.
And if the agent requires multi-step actions to execute and the current performance is satisfactory, do not replace this agent during the process.
- In addition, communication messages between agents do not include the process of them using tools (e.g., searching process). Therefore, do not force them to provide related processes in their communication.
- If certain constraints consistently cannot be met or a consensus cannot be reached, inform the corresponding agent that they can appropriately relax the requirements. The top priority is to submit the plan on time (within 20 turns of conversation).

{{if .refcase}}
## Relevant Case for Reference:
Here you have access to a historical reference case that contains the user's question, the corresponding workflow for that problem, and reflective insights from different agents' experiences. You can reference these relevant experiences to provide better guidance for agent improvement.
{{.refcase}}{{end}}

## Current User's Query:
{{.question}}

## Operational Workflow of Current System:
The multi-agent system you are currently monitoring operates based on the following workflow:
~~~
{{.sop}}
~~~

## Agents Tool Usage History:
All available tools in system are: ["FlightSearch", "GoogleDistanceMatrix", "CitySearch", "AccommodationSearch", "RestaurantSearch", "AttractionSearch"].
Note that each agent will see the execution results, but for the sake of brevity, they are invisible to the Watcher.
~~~
{{.action_history}}
~~~

## Agents Conversation History for Analysis:
Note that:
1. The below history also contains your previous messages of watching and guidance.
2. Every agent only needs to convey the information it provides. There is no need to repeat the information already provided by predecessors.
~~~
{{.history}}
~~~
Please note that communication is still in progress, so it is normal for some messages not to be forwarded or for some agents not to have sent messages yet. The conversation ends when the final plan is sent to the user.

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
`
// Additionally, if the agent requires multi-step actions to execute and the current performance is satisfactory, do not replace this agent during the process.
// If an 'Observation' in the conversation history indicates an error, it should not be attributed to the agent and not be treated as abnormal behavior. 
// However, if multiple instances of Feedback indicate errors, you should regard this as evidence of abnormal behavior on the part of the agent.

const WatchSuffix = `
Now, it is your turn to give your answer. Analyze the provided conversation history and return your JSON response.
If an agent has just completed a search but has not yet had time to forward the message, do not replace this agent!
`

const SOPGeneratorPrompt = `Your task is to act as an expert in designing multi-agent systems to generate a travel plan for the user. You need to generate a Standard Operating Procedure (SOP) in JSON format.

- The system must provide a complete plan for the user, including transportation, restaurant names, accommodation names, and attraction names, even if the user does not explicitly state these requirements.
- There are some important considerations for making the plan:
1.  The itinerary must be a closed loop, meaning user needs to return to starting point on the last day.
2.  Do not revisit any city in the middle of the trip.
3.  If using a self-driving at any point, it is not allowed to use planes or taxis for the entire journey.
4.  Restaurants for each day and each meal must not be repeated.
5.  Attractions for each day must not be repeated.
6.  When arranging accommodations, it must meet the minimum stay requirements of each hotel.
7.  Accommodations, restaurants, and attractions must match the city the user is in on that day. However, if the user have not yet departed or have already returned to starting point, no meals or accommodations need to be arranged.
8.  The information in the plan must strictly match the information found through search, especially for the flight number,the names of hotels, restaurants, and attractions.
9.  The total cost must be within budget, and it can be confirmed that the attractions provided in the search results are all free.
10. The selected room type and constraints must meet the user's conditions (if any).
11. The selected restaurants must cover the cuisines the user wants (if any).
12. The chosen mode of transportation must meet the user's preferences (if any).
13. Use the exact city name without adding its state.
14. The number of people the user initially mentioned is the total number of people, so do not add the number of children to it.

You need to design a SOP, which defines the team of agents, their roles, and their collaboration workflow to solve the user's query.

You must follow the structure of the provided template exactly. The main components of the SOP are:
- "team": A list of agent names that will be part of the team.
- "workflow": A description of the workflow, showing how agents interact with each other.
- "details": A list of objects, where each object defines an agent with:
  - "name": The agent's name (must match a name in the "team" list).
  - "responsibility": A concise description of the agent's main role and purpose. Must start with "You are ……"."
  - "instruction": A detailed guide and important notes on how the agent should perform its task. DO NOT specify the output format for agent. DO NOT include any example in the instruction.
  - "tools": A list of tools that the agents can use to perform its tasks. Available tools are:  ["FlightSearch", "GoogleDistanceMatrix", "CitySearch", "AccommodationSearch", "RestaurantSearch", "AttractionSearch", "CostEnquiry"].

Here is a template for you to follow:
--- TEMPLATE START ---
%s
--- TEMPLATE END ---

**Important Note:** 
The agent instructions within the template contain important information. 
You MUST reuse this information as more as possible. 
In addition to these instructions, you can add new instructions or elaborate on certain instructions based on user needs.
 
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


const SOPGeneratorPrompt_rag = `Your task is to act as an expert in designing multi-agent systems to generate a travel plan for the user. You need to generate a Standard Operating Procedure (SOP) in JSON format.

- The system must provide a complete plan for the user, including transportation, restaurant names, accommodation names, and attraction names, even if the user does not explicitly state these requirements.
- There are some important considerations for making the plan:
1.  The itinerary must be a closed loop, meaning user needs to return to starting point on the last day.
2.  Do not revisit any city in the middle of the trip.
3.  If using a self-driving at any point, it is not allowed to use planes or taxis for the entire journey.
4.  Restaurants for each day and each meal must not be repeated.
5.  Attractions for each day must not be repeated.
6.  When arranging accommodations, it must meet the minimum stay requirements of each hotel.
7.  Accommodations, restaurants, and attractions must match the city the user is in on that day. However, if the user have not yet departed or have already returned to starting point, no meals or accommodations need to be arranged.
8.  The information in the plan must strictly match the information found through search, especially for the flight number, the names of hotels, restaurants, and attractions.
9.  The total cost must be within budget, and it can be confirmed that the attractions provided in the search results are all free.
10. The selected room type and constraints must meet the user's conditions (if any).
11. The selected restaurants must cover the cuisines the user wants (if any).
12. The chosen mode of transportation must meet the user's preferences (if any).
13. Use the exact city name without adding its state.
14. The number of people the user initially mentioned is the total number of people, so do not add the number of children to it.

You need to design a SOP, which defines the team of agents, their roles, and their collaboration workflow to solve the user's query.

You must follow the structure of the provided template exactly. The main components of the SOP are:
- "team": A list of agent names that will be part of the team.
- "workflow": A description of the workflow, showing how agents interact with each other.
- "details": A list of objects, where each object defines an agent with:
  - "name": The agent's name (must match a name in the "team" list).
  - "responsibility": A concise description of the agent's main role and purpose. Must start with "You are ……"."
  - "instruction": A detailed guide and important notes on how the agent should perform its task. DO NOT specify the output format for agent. DO NOT include any example in the instruction.
  - "tools": A list of tools that the agents can use to perform its tasks. Available tools are:  ["FlightSearch", "GoogleDistanceMatrix", "CitySearch", "AccommodationSearch", "RestaurantSearch", "AttractionSearch", "CostEnquiry"].

**Important Note:** 
The agent instructions within the following example contain important information. 
You MUST reuse this information as more as possible. 
In addition to these instructions, you can add new instructions or elaborate on certain instructions based on user needs and important considerations above.

Your entire response MUST be in a single JSON object with the following format. Do not add any text outside of this JSON structure:
~~~
{
  "thought": "Your analysis of the need of user's question and the reasoning for the chosen team and workflow.",
  "content": { ... the complete SOP JSON object goes here ... },
  "cate": "end"
}
~~~

**Example:**
User: "%s"
Output:
{
  "thought": %s,
  "content": %s,
  "cate": "end"
}

User: "%s"
`

const SOPGeneratorPrompt_temp_rag = `Your task is to act as an expert in designing multi-agent systems to generate a travel plan for the user. You need to generate a Standard Operating Procedure (SOP) in JSON format.

- The system must provide a complete plan for the user, including transportation, restaurant names, accommodation names, and attraction names, even if the user does not explicitly state these requirements.
- There are some important considerations for making the plan:
1.  The itinerary must be a closed loop, meaning user needs to return to starting point on the last day.
2.  Do not revisit any city in the middle of the trip.
3.  If using a self-driving at any point, it is not allowed to use planes or taxis for the entire journey.
4.  Restaurants for each day and each meal must not be repeated.
5.  Attractions for each day must not be repeated.
6.  When arranging accommodations, it must meet the minimum stay requirements of each hotel.
7.  Accommodations, restaurants, and attractions must match the city the user is in on that day. However, if the user have not yet departed or have already returned to starting point, no meals or accommodations need to be arranged.
8.  The information in the plan must strictly match the information found through search, especially for the flight number, the names of hotels, restaurants, and attractions.
9.  The total cost must be within budget, and it can be confirmed that the attractions provided in the search results are all free.
10. The selected room type and constraints must meet the user's conditions (if any).
11. The selected restaurants must cover the cuisines the user wants (if any).
12. The chosen mode of transportation must meet the user's preferences (if any).
13. Use the exact city name without adding its state.
14. The number of people the user initially mentioned is the total number of people, so do not add the number of children to it.

You need to design a SOP, which defines the team of agents, their roles, and their collaboration workflow to solve the user's query.

You must follow the structure of the provided template exactly. The main components of the SOP are:
- "team": A list of agent names that will be part of the team.
- "workflow": A description of the workflow, showing how agents interact with each other.
- "details": A list of objects, where each object defines an agent with:
  - "name": The agent's name (must match a name in the "team" list).
  - "responsibility": A concise description of the agent's main role and purpose. Must start with "You are ……"."
  - "instruction": A detailed guide and important notes on how the agent should perform its task. DO NOT specify the output format for agent. DO NOT include any example in the instruction.
  - "tools": A list of tools that the agents can use to perform its tasks. Available tools are:  ["FlightSearch", "GoogleDistanceMatrix", "CitySearch", "AccommodationSearch", "RestaurantSearch", "AttractionSearch", "CostEnquiry"].

Here is a template for you to follow:
--- TEMPLATE START ---
%s
--- TEMPLATE END ---

**Important Note:** 
The agent instructions within the template contain important information. 
You MUST reuse this information as more as possible. 
In addition to these instructions, you can add new instructions or elaborate on certain instructions based on user needs.

Now, analyze the following user question to determine the necessary agents and workflow.

Your entire response MUST be in a single JSON object with the following format. Do not add any text outside of this JSON structure:
~~~
{
  "thought": "Your analysis of the need of user's question and the reasoning for the chosen team and workflow.",
  "content": { ... the complete SOP JSON object goes here ... },
  "cate": "end"
}
~~~

**Example:**
User: "%s"
Output:
{
  "thought": %s,
  "content": %s,
  "cate": "end"
}

User: "%s"
`

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

// {{if .refcase}}
// ## Relevant Case for Reference:
// Here's a historically similar case, which includes the user's query, a reflection on your past actions (if applicable), and the standard plan that the overall system should output (ground truth). 
// You can refer to the relevant experience and the standard plan to better complete your task and avoid repeating the same mistakes.
// {{.refcase}}{{end}}

// refv2
// {{if .refcase}}
// ## Relevant Case for Reference:
// Here's a historically similar case, which includes the user's query, a reflection on your past experience. 
// You can refer to the relevant feedback and improved instructions to better complete your task and avoid repeating the same mistakes.
// {{.refcase}}{{end}}

const NewEndBaseInstructions = `
### Team Members & Collaboration
You are part of a multi-agent system. Your name is {{ .name }} in team. Here is other agents in your team [{{.agent_names}}].
The following is the reference Standard Operating Procedure (SOP) for the task solving process.
{{.sop}}

### Instructions
{{.role}}

### Example of Final Travel Plan
Here is an example of the input query and output plan, and your output travel plan should be similar to the example.
** Example **
Query: Could you create a travel plan for 7 people from Ithaca to Charlotte spanning 3 days, from March 8th to March 14th, 2022, with a budget of $30,200?
Output Travel Plan:
Day 1:
Current City: from Ithaca to Charlotte
Transportation: Flight Number: F3633413, from Ithaca to Charlotte, Departure Time: 05:38, Arrival Time: 07:46
Breakfast: Nagaland's Kitchen, Charlotte
Attraction: The Charlotte Museum of History, Charlotte
Lunch: Cafe Maple Street, Charlotte
Dinner: Bombay Vada Pav, Charlotte
Accommodation: Affordable Spacious Refurbished Room in Bushwick!, Charlotte

Day 2:
Current City: Charlotte
Transportation: -
Breakfast: Olive Tree Cafe, Charlotte
Attraction: The Mint Museum, Charlotte;Romare Bearden Park, Charlotte.
Lunch: Birbal Ji Dhaba, Charlotte
Dinner: Pind Balluchi, Charlotte
Accommodation: Affordable Spacious Refurbished Room in Bushwick!, Charlotte

Day 3:
Current City: from Charlotte to Ithaca
Transportation: Flight Number: F3786167, from Charlotte to Ithaca, Departure Time: 21:42, Arrival Time: 23:26
Breakfast: Subway, Charlotte
Attraction: Books Monument, Charlotte.
Lunch: Olive Tree Cafe, Charlotte
Dinner: Kylin Skybar, Charlotte
Accommodation: -
** End of Example **
Please adhere strictly to the output format above. 
If the mode of travel is self-driving, the 'Transportation' field should be in the following format: 
'Self-driving, from Kansas City to Pensacola, duration: 14 hours 2 mins, distance: 1,433 km, cost: 71'
If the mode of travel is a taxi, the format should be like: 
'Taxi, from State College(Pennsylvania) to Greer, duration: 9 hours 29 mins, distance: 982 km, cost: 982'.

### Current Task & Conversation History:
~~~
{{.history}}
~~~

### Output Format:
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
#### 2. Delivering the Final Answer
When you have gathered all the necessary information and are ready to provide the final travel plan to the user, please use the following format:
~~~
{
  "thought": "Clearly describe your reasoning process.",
  "content": "{Output Travel Plan}."
  "cate": "END",
  "receiver": "User",
}
~~~
`

// const NewEndBaseInstructions = `
// ### Team Members & Collaboration
// You are part of a multi-agent system. Your name is {{ .name }} in team. Here is other agents in your team [{{.agent_names}}].
// The following is the reference Standard Operating Procedure (SOP) for the task solving process.
// {{.sop}}

// ### Instructions
// {{.role}}

// ### Example of Final Travel Plan
// Here is an example of the input query and output plan, and your output travel plan should be similar to the example.
// ** Example **
// Query: Could you create a travel plan for 7 people from Ithaca to Charlotte spanning 3 days, from March 8th to March 14th, 2022, with a budget of $30,200?
// Output Travel Plan:
// Day 1:
// Current City: from Ithaca to Charlotte
// Transportation: Flight Number: F3633413, from Ithaca to Charlotte, Departure Time: 05:38, Arrival Time: 07:46
// Breakfast: Nagaland's Kitchen, Charlotte
// Attraction: The Charlotte Museum of History, Charlotte
// Lunch: Cafe Maple Street, Charlotte
// Dinner: Bombay Vada Pav, Charlotte
// Accommodation: Affordable Spacious Refurbished Room in Bushwick!, Charlotte

// Day 2:
// Current City: Charlotte
// Transportation: -
// Breakfast: Olive Tree Cafe, Charlotte
// Attraction: The Mint Museum, Charlotte;Romare Bearden Park, Charlotte.
// Lunch: Birbal Ji Dhaba, Charlotte
// Dinner: Pind Balluchi, Charlotte
// Accommodation: Affordable Spacious Refurbished Room in Bushwick!, Charlotte

// Day 3:
// Current City: from Charlotte to Ithaca
// Transportation: Flight Number: F3786167, from Charlotte to Ithaca, Departure Time: 21:42, Arrival Time: 23:26
// Breakfast: Subway, Charlotte
// Attraction: Books Monument, Charlotte.
// Lunch: Olive Tree Cafe, Charlotte
// Dinner: Kylin Skybar, Charlotte
// Accommodation: -
// ** End of Example **
// Please adhere strictly to the output format above. 
// If the mode of travel is self-driving, the 'Transportation' field should be in the following format: 
// 'Self-driving, from Kansas City to Pensacola, duration: 14 hours 2 mins, distance: 1,433 km, cost: 71'
// If the mode of travel is a taxi, the format should be like: 
// 'Taxi, from State College(Pennsylvania) to Greer, duration: 9 hours 29 mins, distance: 982 km, cost: 982'.

// ### Current Task & Conversation History:
// ~~~
// {{.history}}
// ~~~

// ### Output Format:
// You must deliver the final plan to the user, and response with json format like below:
// ~~~
// {
//   "thought": "Clearly describe your reasoning process.",
//   "content": "{Output Travel Plan}."
//   "cate": "END",
//   "receiver": "User",
// }
// ~~~
// `

const workflow = `Workflow {
    1. User -> PlanAgent;
    2. PlanAgent -> AnswerAgent;
    3. AnswerAgent -> End;
}`

const NULLSuffix = `
Output:
`

const PlanADescription = `Analyse the given travel planning question and search necessary information.`

const PlanAPrompt = `
You are a travel planning assistant. Your task is to carefully analyze the user's travel needs, use the provided tools to gather the necessary information, and compile all relevant information to send to the final travel plan generator.`

const AnswerADescription = `Generate the final travel plan based on gathered information.`

const AnswerAPrompt = `
You are the Travel Plan Generator Agent.
Your core mission is to generate the final, comprehensive travel plan for the user's travel planning question.
Based on the provided information and query, please generate a detailed plan, including specifics such as flight numbers (e.g., F0123456), restaurant names, and accommodation names. Note that all the information in your plan should be derived from the provided data. You must adhere to the format given in the example. Additionally, all details should align with commonsense. The symbol '-' indicates that information is unnecessary. For example, in the provided sample, you do not need to plan after returning to the departure city. When you travel to two cities in one day, you should note it in the 'Current City' section as in the example (i.e., from A to B).
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
Please provide a comprehensive travel plan as your final answer, including detailed recommendations for accommodations, transportation, attractions, restaurants, and daily schedules based on the user's requirements and budget.
`
