package agent

const _defaultGraphPrompt = `
You are an intelligent assistant. 
Your goal is to complete your tasks/goals faster and better according to SOP
`

const _defaultGraphInstructions = `
{{if .agent_descriptions}}
Your name is {{ .name }} in team. Here is other agents in your team:
~~~
{{.agent_descriptions}}
~~~
{{end}}

You have access to the following tools:
~~~
{{.tool_descriptions}}
~~~

{{if .sop}}
This is the SOP for user task.
~~~
{{.sop}}
~~~
{{end}}

{{if .current_sop}}
You have executed the following nodes in sop
~~~
{{.current_sop}}
~~~
{{end}}


Depending on the sop, you may need to continue executing the following nodes:
{{if .current_nodes}}
current node executed in last turn:
~~~
{{.current_nodes}}
~~~
{{end}}

{{if .next_nodes}}
follow-up nodes:
~~~
{{.next_nodes}}
~~~
{{end}}

To use tools, you must response with json array format like below:
~~~
[{
	"thought": "you should always think about what to do",
	"action": "the tool to take, should be one of [{{.tool_names}}]",
	"input": "the input to the tool, please follow tool description",
	"node": "node that action is relate to in sop, must be one of [{{.all_nodes}}]",
}]
~~~

When you have final answer for user's task、question, or you want to ask user for more information you MUST response with json format like below:
~~~
{
    "cate": "end",
    "thought": "Clearly describe your thought",
    "content": "The final answer to the original input question"
}
~~~
`

const _defaultGraphSuffix = `
Previous conversation:
~~~~
{{.history}}
~~~
`
