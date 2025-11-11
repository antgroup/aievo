package main

const (
	ReflectionPrompt = `You are an expert in analyzing and refining multi-agent systems for code generation tasks. Your task is to reflect on a code generation attempt by a team of agents and identify areas for improvement.
Your need to identify the root causes of issues and provide concrete, actionable feedback to improve the system's performance for future coding tasks.
Pay special attention to:
- Code correctness and syntax errors.
- Failure to pass provided test cases.
- Communication efficiency between agents.

## Current Task
**1. The User's Coding Problem:**
%s

**2. Standard Answer:**
%s

**3. System Generated Code:**
%s

**4. Evaluation Results:**
%s

**5. The Standard Operating Procedure (SOP) that was used:**
%s

**6. The full communication history of the agent team during the attempt:**
%s

## Importan Note:
1. Do not directly reveal the correct answer. Instead, you need to elevate the overall collaboration process by guiding the agent through instructions to complete the answer better.
2. You need to make the agent understand that the example provided by the user is extremely important, and its correctness should not be questioned.

## Ouput Requirements
Your output must follow the JSON format below. Do not add any text outside the JSON structure.
**Output Format (JSON):**
{
  "thought": "Your analysis of the root cause of failure. Analyze the entire process, from coding to execution, and summarize the primary reason for the failure here.",
  "content": {
    "failure_reason": "A concise summary of the primary reason for the failure (e.g., logical error, failed test case).",
    "sop_critique": {
      "weaknesses": "Identify specific weaknesses in the provided SOP. Did it lack a necessary role? Was the workflow inefficient? Were the instructions not clear?",
      "suggestions": "Provide concrete suggestions for improving the SOP. This should be a list of changes, not a new SOP."
    },
    "agent_guidance": [
      {
        "agent_name": "Name of the agent",
        "feedback": "Specific feedback for this agent. What did it do wrong?",
        "new_instruction": "Some new instructions for this agent that would guide it to avoid the same mistakes, which should be concise and actionable."
      }
    ]
  },
  "cate": "end"
}
`
)


// ## Example of Input-Output
// ~~~
// Input:
// **1. The User's Coding Problem:**
// from typing import List

// def has_close_elements(numbers: List[float], threshold: float) -> bool:
//     """ Check if in given list of numbers, are any two numbers closer to each other than
//     given threshold.
//     >>> has_close_elements([1.0, 2.0, 3.0], 0.5)
//     False
//     >>> has_close_elements([1.0, 2.8, 3.0, 4.0, 5.0, 2.0], 0.3)
//     True
//     """

// **2. System Generated Code:**
//     for i in range(len(numbers)):
//         for j in range(len(numbers)):
//             distance = abs(numbers[i] - numbers[j])
//             if distance < threshold:
//                 return True
//     return False

// **3. Evaluation Results and Constraint Violations:**
// {"test_results": "Failed: The code returns True for the first example, but it should be False because the comparison includes the element with itself (distance is 0).", "success": false}

// **4. The Standard Operating Procedure (SOP) that was used:**
// Workflow:\n1. User -> CoderAgent;\n2. CoderAgent -> AnswerAgent;\n3. AnswerAgent -> End.\n\nDescription: ……

// **5. The full communication history of the agent team during the planning attempt:**
// (For brevity, the communication history is omitted here)

// Output:
// ** Reflection Content: **
// {
//   "failure_reason": "The generated code incorrectly compares an element with itself, leading to a wrong result when the threshold is positive.",
//   "sop_critique": {
//     "weaknesses": "The instructions for the CoderAgent are too generic and do not emphasize the need to handle edge cases, such as an element being compared to itself.",
//     "suggestions": "Update the CoderAgent's instructions to explicitly mention checking for and excluding self-comparison in loops."
//   },
//   "agent_guidance": [
//     {
//       "agent_name": "CoderAgent",
//       "feedback": "The agent's code failed because it didn't account for the case where i == j in the nested loops.",
//       "new_instruction": "When iterating through a list to compare pairs of elements, make sure to add a condition to skip the case where an element is compared with itself (e.g., if i != j)."
//     }
//   ]
// }
// ~~~

const (
	RevisionPrompt = `You are an expert multi-agent system designer for code generation.
The system is designed to provide a complete and correct Python function based on a user's problem description.
There is a team of agents working together to write code. The agents can use a 'bash' tool to execute code and tests. The agents must follow a Standard Operating Procedure (SOP) that defines their roles, instructions, and workflow.
Your task is to revise a past SOP based on a critical reflection of a past failure.

The main components of the SOP are:
- "team": A list of agent names that will be part of the team.
- "workflow": A description of the workflow, showing how agents interact with each other.
- "details": A list of objects, where each object defines an agent with:
  - "name": The agent's name (must match a name in the "team" list).
  - "responsibility": A concise description of the agent's main role and purpose. Must start with "You are ……"."
  - "instruction": A detailed guide and important notes on how the agent should perform its task. DO NOT specify the output format for agent. DO NOT include any example in the instruction.
  - "tools": A list of tools that the agents can use. The only available tool is ["bash"].

Here is a template for you to reference the format of the SOP:
--- TEMPLATE START ---
%s
--- TEMPLATE END ---

You will be given a user's query, the original SOP and a detailed analysis of why it failed. Your goal is to produce a new, improved SOP that addresses these failures and is more robust for similar tasks in the future.

**1. User's Query and Original SOP:**
%s

**2. Reflection on Failure:**
%s

**Your Task:**

Generate a new SOP in the exact same JSON format as the original. The new SOP should incorporate the lessons from the reflection.
- You may add, remove, or redefine the agent in the team.
- You may refine the workflow field.
- You must provide clearer, more precise instructions for the agent in the "details" section.

Your entire response MUST be in a single JSON object with the following format. Do not add any text outside of this JSON structure:
~~~
{
  "thought": "Your analysis of the user's coding problem and the reasoning for the revised team and workflow based on the reflection.",
  "content": { ... the complete SOP JSON object goes here ... },
  "cate": "end"
}
~~~
`
)


// **3. Standard Answer for the User's Query:**
// %s