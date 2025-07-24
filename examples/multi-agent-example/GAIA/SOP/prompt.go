package main

const (
	ReflectionPrompt = `You are an expert in analyzing and refining multi-agent systems. Your task is to reflect on a failed attempt by a team of agents to answer a given question. I will provide you with all the necessary context.

Your goal is to identify the root causes of the failure and provide concrete, actionable feedback to improve the system's performance for future attempts.

Here is the context for the failed task:

**1. The User's Question:**
%s

**2. Ground Truth for Verification:**
- **Correct Answer:** %s
- **Human-Annotated Steps to Solve (for reference):** %s

**3. The Standard Operating Procedure (SOP) that was used:**
%s

**4. The full communication history of the agent team during the failed attempt:**
%s

**Your Reflection Task:**

Based on all the information above, please perform a thorough analysis and provide your reflection.
Please note that the agent can only use the standard Google web search tool and cannot access APIs from other websites. Therefore, instead of suggesting the use of specific webpage APIs, please guide the agent on how to better search for results using Google search.
**Important:** When providing critiques and guidance for the SOP and agents, you must not disclose any facts from the 'Ground Truth' answer. Your goal is to refine the problem-solving *process*, not to hint at the solution.
Your output must follow the JSON format below. Do not add any text outside the JSON structure.

**Output Format (JSON):**
{
  "cate": "end",
  "thought": "Your analysis of the root cause of failure. Analyze the entire process, from planning to execution, and summarize the primary reason for the failure here.",
  "content": {
    "failure_reason": "A concise summary of the primary reason for the failure.",
    "sop_critique": {
      "weaknesses": "Identify specific weaknesses in the provided SOP. Did it lack a necessary role? Was the workflow inefficient or illogical? Were the responsibilities not clearly defined?",
      "suggestions": "Provide concrete suggestions for improving the SOP. This should not be a new SOP, but a list of changes. For example: 'Add a verification step after the WebSearcher', or 'Merge the Planner and Summarizer roles for simpler tasks'."
    },
    "agent_guidance": [
      {
        "agent_name": "Name of the first agent (e.g., Planner)",
        "feedback": "Specific feedback for this agent. What did it do wrong? How could it have performed better?",
        "revised_instruction": "A revised 'instruction' for this agent that would guide it to perform better on this specific task. This should be a direct, actionable instruction."
      },
      {
        "agent_name": "Name of the second agent (e.g., WebSearcher)",
        "feedback": "...",
        "revised_instruction": "..."
      }
    ]
  }
}
`
)

const (
	RevisionPrompt = `You are an expert multi-agent system designer.
Your task is to revise a Standard Operating Procedure (SOP) based on a critical reflection of a past failure.

You will be given the original SOP and a detailed analysis of why it failed. Your goal is to produce a new, improved SOP that addresses these failures and is more robust for similar tasks in the future.

**1. Original SOP:**
%s

**2. Reflection on Failure:**
%s

**Your Task:**

Generate a new SOP in the exact same JSON format as the original. The new SOP should incorporate the lessons from the reflection.
- You may need to add, remove, or redefine agent roles.
- You may refine the workflow (the "sop" field).
- You must provide clearer, more precise instructions for each agent in the "details" section.
- Ensure the "tools" for each agent are appropriate and sufficient.

Your entire response MUST be in a single JSON object with the following format. Do not add any text outside of this JSON structure:
~~~
{
  "cate": "end"
  "thought": "Your analysis of the need of user's question and the reasoning for the chosen team and workflow.",
  "content": { ... the complete SOP JSON object goes here ... },
}
~~~
`
)
