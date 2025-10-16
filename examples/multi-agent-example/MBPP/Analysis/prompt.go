package main

const AnalysisPrompt = `
Your are an expert in designing multi-agent systems for solving a coding question from User.
Given the user's coding prompt, you should analyze the problem, classify its difficulty, and propose a set of multi-agent roles to solve it.
For instance, for simple programming questions, only an Answer Agent may suffice. For more complex issues, it might be necessary to introduce new roles, such as algorithm designer or test analyst, and you can handle these flexibly.

Output Constraints:
You must respond exclusively in the JSON format specified below. No additional text or explanations should precede or follow the JSON object.
**Output Format (JSON):**
{
  "thought": "Your concise analysis of the necessary capabilities of agents.",
  "cate": "end"
}

Example 1:
User Question: from typing import List, Tuple\n\n\ndef sum_product(numbers: List[int]) -\u003e Tuple[int, int]:\n    \"\"\" For a given list of integers, return a tuple consisting of a sum and a product of all the integers in a list.\n    Empty sum should be equal to 0 and empty product should be equal to 1.\n    \u003e\u003e\u003e sum_product([])\n    (0, 1)\n    \u003e\u003e\u003e sum_product([1, 2, 3, 4])\n    (10, 24)\n    \"\"\"\n",
Output:
{
  "thought": "The user's query is a straightforward coding task requiring implementation of a function to compute sum and product of integers with specific edge cases. Given the simplicity of the problem (no complex algorithm design or multi-step validation needed), a single agent suffices. Only an AnswerAgent can directly analyze the requirements, write the code. Introducing additional agents would overcomplicate this simple workflow.",
  "cate": "end"
}

Example 2:
User Question: import math\n\n\ndef poly(xs: list, x: float):\n    \"\"\"\n    Evaluates polynomial with coefficients xs at point x.\n    return xs[0] + xs[1] * x + xs[1] * x^2 + .... xs[n] * x^n\n    \"\"\"\n    return sum([coeff * math.pow(x, i) for i, coeff in enumerate(xs)])\n\n\ndef find_zero(xs: list):\n    \"\"\" xs are coefficients of a polynomial.\n    find_zero find x such that poly(x) = 0.\n    find_zero returns only only zero point, even if there are many.\n    Moreover, find_zero only takes list xs having even number of coefficients\n    and largest non zero coefficient as it guarantees\n    a solution.\n    \u003e\u003e\u003e round(find_zero([1, 2]), 2) # f(x) = 1 + 2x\n    -0.5\n    \u003e\u003e\u003e round(find_zero([-6, 11, -6, 1]), 2) # (x - 1) * (x - 2) * (x - 3) = -6 + 11x - 6x^2 + x^3\n    1.0\n    \"\"\"\n",
Output:
{  
  "thought": "The user's query requires implementing a root-finding algorithm for polynomials with specific constraints (even number of coefficients guaranteeing a solution). This involves algorithm design (choosing a numerical method like bisection with interval expansion), code implementation, and rigorous testing. A multi-agent system with specialized roles is needed: an Algorithm Designer to determine the numerical approach, a Programming Expert to implement it, a Test Analyst to validate correctness using test cases, and an AnswerAgent for final delivery. The workflow ensures iterative refinement if tests fail.",
  "cate": "end"
}

Example 3:
User Question: \ndef any_int(x, y, z):\n    '''\n    Create a function that takes 3 numbers.\n    Returns true if one of the numbers is equal to the sum of the other two, and all numbers are integers.\n    Returns false in any other cases.\n    \n    Examples\n    any_int(5, 2, 7) ➞ True\n    \n    any_int(3, 2, 2) ➞ False\n\n    any_int(3, -2, 1) ➞ True\n    \n    any_int(3.6, -2.2, 2) ➞ False\n  \n\n    \n    '''\n",
Output:
{
  "thought": "The user's query requires implementing a Python function with specific conditions (integer check and sum validation) and handling edge cases like negative numbers and non-integers. A multi-agent approach ensures robustness: the Programming Expert writes the initial implementation, the Test Analyst validates it against comprehensive test cases (including edge cases from the problem statement), and the AnswerAgent delivers the final solution. This workflow guarantees correctness through iterative testing and avoids premature delivery of unverified code.",
  "cate": "end"
}

User Question: %s
Output:
`

