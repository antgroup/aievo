package main

const (
	ReflectionPrompt = `You are an expert in analyzing and refining multi-agent systems for travel planning tasks. Your task is to reflect on a travel planning attempt by a team of agents and identify areas for improvement.

Your goal is to identify the root causes of issues and provide concrete, actionable feedback to improve the system's performance for future travel planning tasks.

Here is the context for the travel planning task:

**1. The User's Travel Request:**
%s

**2. Expected Travel Plan (Ground Truth):**
%s

**3. System Generated Travel Plan:**
%s

**4. Evaluation Results and Constraint Violations:**
%s

**5. The Standard Operating Procedure (SOP) that was used:**
%s

**6. The full communication history of the agent team during the planning attempt:**
%s

**Your Reflection Task:**

Based on all the information above, please perform a thorough analysis and provide your reflection.
The agents have access to various travel tools including flight search, accommodation search, restaurant search, attraction search, distance calculation, and cost inquiry tools.
**Important:** When providing critiques and guidance for the SOP and agents, focus on improving the problem-solving *process* and constraint satisfaction, not just providing the correct answer.

Pay special attention to:
- Constraint violations shown in the evaluation results
- Logic and feasibility issues in the travel plan
- Proper use of available travel tools
- Communication efficiency between agents
- Budget management and optimization

Your output must follow the JSON format below. Do not add any text outside the JSON structure.

**Output Format (JSON):**
{
  "thought": "Your analysis of the root cause of failure. Analyze the entire process, from planning to execution, and summarize the primary reason for the failure here.",
  "content": {
    "failure_reason": "A concise summary of the primary reason for the failure.",
    "sop_critique": {
      "weaknesses": "Identify specific weaknesses in the provided SOP. Did it lack a necessary role? Was the workflow inefficient or illogical? Were the responsibilities not clearly defined?",
      "suggestions": "Provide concrete suggestions for improving the SOP. This should not be a new SOP, but a list of changes. For example: 'Add a verification step after the WebSearcher', or 'Merge the Planner and Summarizer roles for simpler tasks'."
    },
    "agent_guidance": [
      {
        "agent_name": "Name of the first agent",
        "feedback": "Specific feedback for this agent. What did it do wrong? How could it have performed better?",
        "revised_instruction": "A revised 'instruction' for this agent that would guide it to perform better on this specific task. This should be a direct, actionable instruction."
      },
      {
        "agent_name": "Name of the second agent",
        "feedback": "...",
        "revised_instruction": "..."
      }
    ]
  },
  "cate": "end"
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
  "thought": "Your analysis of the need of user's question and the reasoning for the chosen team and workflow.",
  "content": { ... the complete SOP JSON object goes here ... },
  "cate": "end"
}
~~~
`
)
