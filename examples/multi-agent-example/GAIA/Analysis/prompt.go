package main

const AnalysisPrompt = `
Your task is to act as an expert in designing multi-agent systems for solving user's question. 
Based on the user's question, You need to analyze the required abilities or tools of the question, which will guide the subsequent agent team building (you do not need to do this).

Examples of abilities and tools include, but are not limited to:
- WebSearch: For accessing real-time or publicly available information.
- FileAnalyzer: For parsing and extracting data from local files.

Output Constraints:
You must respond exclusively in the JSON format specified below. No additional text or explanations should precede or follow the JSON object.
**Output Format (JSON):**
{
  "thought": "Your concise analysis of the necessary capabilities of agents.",
  "cate": "end"
}

**Example:**
Example 1:
User Question: As of May 2023, how many stops are between South Station and Windsor Gardens on MBTA’s Franklin-Foxboro line (not included)?
Output:
{
  "thought": "The user is asking for a precise, time-sensitive piece of information regarding public transit. This requires external data retrieval. The plan is to first use a WebSearcher agent to find the official MBTA Franklin-Foxboro line schedule or route map as of May 2023. A DataExtractor agent would then process the retrieved information to identify the sequence of stops and count the intermediate stations between South Station and Windsor Gardens.",
  "cate": "end"
}

Example 2:
User Question: An office held a Secret Santa gift exchange where each of its twelve employees was assigned one other employee in the group to present with a gift. Each employee filled out a profile including three likes or hobbies. On the day of the gift exchange, only eleven gifts were given, each one specific to one of the recipient's interests. Based on the information in the document, who did not give a gift?\nFILENAME:cffe0e32-c9a6-4c52-9877-78ceb4aaa9fb.docx",
Output:
{  
  "thought": "The question requires analyzing a document to determine which employee did not give a gift in a Secret Santa exchange. Since the document (FILENAME provided) contains the necessary data (employee profiles and gift details), a FileAnalyzer is essential. The Planner creates a strategy for extracting relevant information, and the Summarizer synthesizes findings to identify the missing gift. No external search is needed as all data is in the provided file.",
  "cate": "end"
}
 
Example 3:
User Question: What is the average number of pre-2020 works on the open researcher and contributor identification pages of the people whose identification is in this file?\nFILENAME:bec74516-02fc-48dc-b202-55e78d0e17cf.jsonld
Output:
{
  "thought": "The user's question requires a combination of file processing, web interaction, and data computation. The workflow necessitates a FileAnalyzer agent to parse the local 'bec74516-02fc-48dc-b202-55e78d0e17cf.jsonld' file and extract all ORCID identifiers. Subsequently, a WebSearcher or API agent will need to access the ORCID page for each identifier. Finally, a DataProcessor agent will be required to parse the works on each page, filter for publications dated before 2020, count them, and then compute the overall average.",
  "cate": "end"
}

User Question: %s
Output:
`
