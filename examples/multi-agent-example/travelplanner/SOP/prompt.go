package main

const (
	ReflectionPrompt = `You are an expert in analyzing and refining multi-agent systems for travel planning tasks. Your task is to reflect on a travel planning attempt by a team of agents and identify areas for improvement.
Your goal is to identify the root causes of issues and provide concrete, actionable feedback to improve the system's performance for future travel planning tasks.
Pay special attention to:
- Constraint violations shown in the evaluation results
- Communication efficiency between agents

## Example
~~~
**1. The User's Travel Request:**
Can you devise a travel plan that departs from Memphis and includes 2 cities in Pennsylvania? The trip is scheduled for 5 days from March 22nd to March 26th, 2022. The trip's total budget is $1,400.

**2. System Generated Travel Plan:**
[{"day": 1, "current_city": "from Memphis to Philadelphia", "transportation": "Flight Number: AA123, from Memphis to Philadelphia, Departure Time: -, Arrival Time: -", "breakfast": "Asian Chopstick, Philadelphia", "attraction": "Liberty Bell, Philadelphia;", "lunch": "Bangla Sweet Corner, Philadelphia", "dinner": "Gurdas Ram Jalebi Wala, Philadelphia", "accommodation": "Top floor of an amazing duplex, Philadelphia"}, 
{"day": 2, "current_city": "from Philadelphia to Pittsburgh", "transportation": "Flight Number: DL456, from Philadelphia to Pittsburgh, Departure Time: -, Arrival Time: -", "breakfast": "Gian Ji Punjabi Dhaba, Pittsburgh", "attraction": "Point State Park, Pittsburgh;", "lunch": "Costa Coffee, Pittsburgh", "dinner": "Modi's Baker's Zone, Pittsburgh", "accommodation": "Sunny 1 BR Brooklyn Loft Apartment, Pittsburgh"}, 
{"day": 3, "current_city": "Pittsburgh", "transportation": "-", "breakfast": "Beijing Cafe, Pittsburgh", "attraction": "Randyland, Pittsburgh;", "lunch": "Lasha Chinese Food, Pittsburgh", "dinner": "Burger Factory, Pittsburgh", "accommodation": "Sunny 1 BR Brooklyn Loft Apartment, Pittsburgh"}, 
{"day": 4, "current_city": "Pittsburgh", "transportation": "-", "breakfast": "Onyx Bar, Pittsburgh", "attraction": "West End Overlook Park, Pittsburgh;", "lunch": "Moon of Taj, Pittsburgh", "dinner": "China Fare, Pittsburgh", "accommodation": "Sunny 1 BR Brooklyn Loft Apartment, Pittsburgh"}, 
{"day": 5, "current_city": "from Pittsburgh to Memphis", "transportation": "Flight Number: UA789, from Pittsburgh to Memphis, Departure Time: -, Arrival Time: -", "breakfast": "Krishna Yummy Foods, Pittsburgh", "attraction": "-", "lunch": "Maharaja Bhog, Pittsburgh", "dinner": "-", "accommodation": "-"}]

**3. Evaluation Results and Constraint Violations:**
{"commonsense_constraint": 
  {"is_reasonalbe_visiting_city": [true, null], "is_valid_restaurants": [true, null], "is_valid_attractions": [true, null], "is_valid_accommodation": [false, "The accommodation Sunny 1 BR Brooklyn Loft Apartment, Pittsburgh do not obey the minumum nights rule."], "is_valid_transportation": [true, null], "is_valid_information_in_current_city": [true, null], "is_valid_information_in_sandbox": [false, "The flight number in day 1 is invalid in the sandbox."], "is_not_absent": [true, null]}, 
"hard_constraint": null} (Since there are some invalid information in the sandbox, hard constraint can not be measured.)

**4. The Standard Operating Procedure (SOP) that was used:**
Workflow:\n1. User -> Transportation Planner;\n2. Transportation Planner -> Accommodation Planner;\n3. Accommodation Planner -> Restaurant Planner\n   Accommodation Planner -> Attractions Planner (in parallel);\n4. Restaurant Planner, Attractions Planner -> Final Plan Summarizer;\n5. Final Plan Summarizer -> End.\n\nDescription: ……

**5. The full communication history of the agent team during the planning attempt:**
(For brevity, the communication history is omitted here)

** Reflection Content: **
{
  "failure_reason": "1. The flight number is invalid in the sandbox.  2. The accommodation does not obey the minumum nights rule.",
  "sop_critique": {
    "weaknesses": "The instructions for the Transportation Planner did not emphasize the importance of verifying the validity of transportation details against the sandbox data. The Accommodation Planner's instructions lacked clarity on adhering to accommodation constraints, such as minimum stay requirements.",
    "suggestions": "1. Update the Transportation Planner's instructions to include a mandatory use of provided tools to check all transportation details. 2. Revise the Accommodation Planner's instructions to explicitly state the need to comply with all accommodation constraints, including minimum stay requirements."
  },
  "agent_guidance": [
    {
      "agent_name": "Transportation Planner",
      "feedback": "The agent failed to ensure that the flight number provided was valid according to the sandbox data.",
      "new_instruction": "If plan to take the plane, always use the FlightSearch tool to verify the validity of all transportation details, including flight numbers, departure times, and arrival times, against the sandbox data before finalizing any transportation plans."
    },
    {
      "agent_name": "Accommodation Planner",
      "feedback": "The agent did not adhere to the minimum stay requirements for the selected accommodation.",
      "new_instruction": "Always review and comply with all accommodation constraints, including minimum stay requirements, before finalizing any accommodation bookings."
    }
  ]
}
~~~


## Current Task
**1. The User's Travel Request:**
%s

**2. System Generated Travel Plan:**
%s

**3. Evaluation Results and Constraint Violations:**
%s

**4. The Standard Operating Procedure (SOP) that was used:**
%s

**5. The full communication history of the agent team during the planning attempt:**
%s

Your output must follow the JSON format below. Do not add any text outside the JSON structure.
**Output Format (JSON):**
{
  "thought": "Your analysis of the root cause of failure. Analyze the entire process, from planning to execution, and summarize the primary reason for the failure here.",
  "content": {
    "failure_reason": "A concise summary of the primary reason for the failure.",
    "sop_critique": {
      "weaknesses": "Identify specific weaknesses in the provided SOP. Did it lack a necessary role? Was the workflow inefficient or illogical? Were the instructions not clearly defined?",
      "suggestions": "Provide concrete suggestions for improving the SOP. This should not be a new SOP, but a list of changes."
    },
    "agent_guidance": [
      {
        "agent_name": "Name of the agent",
        "feedback": "Specific feedback for this agent. What did it do wrong?",
        "new_instruction": "Some new instructions for this agent that would guide it to avoid the same mistakes, which should be concise, actionable."
      },
      {
        "agent_name": "Name of the agent",
        "feedback": "...",
        "new_instruction": "..."
      }
    ]
  },
  "cate": "end"
}
`
)

const (
	RevisionPrompt = `You are an expert multi-agent system designer.
The system is designed to provide a complete plan for the user, including transportation, restaurant names, accommodation names, and attraction names, even if the user does not explicitly state these requirements.
- There are some important considerations for making the plan, which must be strictly followed by all agents in the team:
1.  The itinerary must be a closed loop, meaning user needs to return to starting point on the last day.
2.  Do not revisit any city in the middle of the trip.
3.  If using a self-driving at any point, it is not allowed to use planes or taxis for the entire journey.
4.  Restaurants for each day and each meal must not be repeated.
5.  Attractions for each day must not be repeated.
6.  When arranging accommodations, it must meet the minimum stay requirements of each hotel.
7.  Accommodations, restaurants, and attractions must match the city you are in on that day. However, if the user have not yet departed or have already returned to starting point, no meals or accommodations need to be arranged.
8.  The information in the plan must strictly match the information found through search, especially for the names of hotels, restaurants, and attractions.
9.  The total cost must be within budget, and all attractions are free here.
10. The selected room type and constraints must meet the user's conditions (if any).
11. The selected restaurants must cover the cuisines the user wants (if any).
12. The chosen mode of transportation must meet the user's preferences (if any).

There is a team of agents working together to create a travel plan based on a user's request. The agents can use various tools to search necessary information. The agents must follow a Standard Operating Procedure (SOP) that defines their roles, instructions, and workflow.
Your task is to revise a past Standard Operating Procedure (SOP) based on a critical reflection of a past failure.
The main components of the SOP are:
- "team": A list of agent names that will be part of the team.
- "workflow": A description of the workflow, showing how agents interact with each other.
- "details": A list of objects, where each object defines an agent with:
  - "name": The agent's name (must match a name in the "team" list).
  - "responsibility": A concise description of the agent's main role and purpose. Must start with "You are ……"."
  - "instruction": A detailed guide and important notes on how the agent should perform its task. DO NOT specify the output format for agent. DO NOT include any example in the instruction.
  - "tools": A list of tools that the agents can use to perform its tasks. Available tools are:  ["FlightSearch", "GoogleDistanceMatrix", "CitySearch", "AccommodationSearch", "RestaurantSearch", "AttractionSearch", "CostEnquiry"].

You will be given the original SOP and a detailed analysis of why it failed. Your goal is to produce a new, improved SOP that addresses these failures and is more robust for similar tasks in the future.

**1. Original SOP:**
%s

**2. Reflection on Failure:**
%s

**Your Task:**

Generate a new SOP in the exact same JSON format as the original. The new SOP should incorporate the lessons from the reflection.
- You may need to add, remove, or redefine the agent in the team.
- You may refine the workflow field.
- You must provide clearer, more precise instructions for each agent in the "details" section. 
**Important Note:** 
The original agent instructions contain important information. You should reuse this information as more as possible, In addition to these instructions, you can add new instructions or elaborate on certain instructions based on the reflection and user query.

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
