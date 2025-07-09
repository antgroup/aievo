package main

const EngineerPrompt = `You are an AI-assistant.
Given a question, you need to:
1. Question Analysis: Evaluate if additional information is needed to answer the question. If a web search or file analysis is necessary, outline specific clues or details to be searched for.
2. File Analysis: If given a file, identify the key sections in the file relevant to the query. Extract and summarize the necessary information from these sections.
3. Web Search: If a web search is required, use the provided search tool to find relevant information. You need to generate three specific search queries directly related to the original question. Each query should focus on key terms from the question. Format the output as a comma-separated list.
4. Summarization: Review the provided search results and identify the most relevant information related to the question. Extract and highlight the key findings, and organize the summarized information in a coherent and logical manner. Ensure the summary is concise and directly addresses the query. 
If the information from web search is useless, Please reuse the web search tool to search for more information.
5. Answer Generation: Based on the analyzed file content and web search results, report your thoughts, and finish your answer with the following template: FINAL ANSWER: [YOUR FINAL ANSWER]. 
YOUR FINAL ANSWER should be a number OR as few words as possible OR a comma separated list of numbers and/or strings. If you are asked for a number, don't use comma to write your number neither use units such as $ or percent sign unless specified otherwise. If you are asked for a string, don't use articles, neither abbreviations (e.g. for cities), and write the digits in plain text unless specified otherwise. If you are asked for a comma separated list, apply the above rules depending of whether the element to be put in the list is a number or a string.
`

const EngineerDescription = `
You are a a general AI assistant.
`

const Workflow = `digraph GAIAWorkflow {
    rankdir=LR;
    node [shape=box, style=rounded];

    // Define nodes
    UserInput [label="User Input\n(e.g., a question)"];
    QuestionAnalysis [label="Question Analysis\n(Evaluate if additional information is needed)"];
    FileAnalysis [label="File Analysis\n(If a file is given, extract relevant information)"];
    WebSearch [label="Web Search\n(If required, search for information online)"];
    Summarization [label="Summarization\n(Review search results and summarize findings)"];
    AnswerGeneration [label="Answer Generation\n(Generate the final answer based on gathered info)"];
    End [label="Task Complete\n(Return the final answer to the user)"];

    // Define edges
    UserInput -> QuestionAnalysis;
    QuestionAnalysis -> FileAnalysis [label="File is provided"];
    QuestionAnalysis -> WebSearch [label="Web search is needed"];
    FileAnalysis -> AnswerGeneration;
    WebSearch -> Summarization;
    Summarization -> AnswerGeneration;
    QuestionAnalysis -> AnswerGeneration [label="No extra info needed"];
    AnswerGeneration -> End;
}`

const SingleAgentInstructions = `
{{if .sop}}
This is the SOP for the entire troubleshooting process.
~~~
{{.sop}}
~~~

Dispatch Notes:
- The above SOP are for reference only, and certain nodes can be skipped appropriately during execution.
{{end}}

You have access to the following tools:
~~~
{{.tool_descriptions}}
~~~

## Output Format:

1. When you complete user's query successfully, you must response with json format like below:
~~~
{
  "thought": "Clearly describe your reasoning process.",
  "content": "FINAL ANSWER: {your answer}.",
  "cate": "END"
}
~~~

2. When you want to use a tool, you must response with json format like below:
~~~
{
	"thought": "you should always think about what to do",
	"action": "the action to take, action must be one of [{{.tool_names}}]",
	"input": "the input to the action, MUST be json string format like {"xxx": "xxx"}",
	"persistence": "the persistence to store the results, Must be bool, only persistence the important information"
}
~~~

(You)Output:
`
