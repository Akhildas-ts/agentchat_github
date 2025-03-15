package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

func (a *GitHubAgent) ProcessQuery(query, projectID, branch string) (string, error) {
	const maxRetries = 3
	const baseDelay = time.Second // Initial retry delay

	var plan []Step
	var err error

	// Retry ProposePlan
	for attempt := 1; attempt <= maxRetries; attempt++ {
		plan, err = a.ProposePlan(query, projectID, branch)
		if err == nil {
			break
		}
		fmt.Printf("Attempt %d: Plan generation failed.......: %v\n", attempt, err)
		time.Sleep(baseDelay * time.Duration(attempt)) // Exponential backoff
	}
	if err != nil {
		return "", fmt.Errorf("plan generation failed after %d attempts: %v", maxRetries, err)
	}

	fmt.Println("plan completed", plan)

	var state *AgentState

	// Retry ExecutePlan
	for attempt := 1; attempt <= maxRetries; attempt++ {

		fmt.Println("ExecutePlan start", plan)
		state, err = a.ExecutePlan(plan, query, projectID, branch)
		if err == nil {
			break
		}
		fmt.Printf("Attempt %d: Plan execution failed: %v\n", attempt, err)
		time.Sleep(baseDelay * time.Duration(attempt)) // Exponential backoff

		fmt.Println("ExecutePlan start", plan)
	}
	if err != nil {
		return "", fmt.Errorf("plan execution failed after %d attempts:.............. %v", maxRetries, err)
	}

	fmt.Println("state", state)

	// Return final answer or fallback
	if state.FinalAnswer != "" {
		return state.FinalAnswer, nil
	}
	return "No answer could be generated.", nil
}

func (a *GitHubAgent) ProposePlan(query, projectID, branch string) ([]Step, error) {
	prompt := fmt.Sprintf(`You are a strategic planning assistant for GitHub project analysis. The user from repository %s (branch: %s) has asked:<br>
%s<br><br>

Use these available steps:<br>
- search_by_query: to search for relevant tasks using vector similarity<br>
- get_global_context: to get summaries of tasks in the project<br>
- combine: to aggregate information from multiple tasks<br>
- invoke_llm: to generate the final response<br><br>

Respond with a JSON array of steps. Example:<br>
[
    {"type": "search_by_query", "args": {"query": "UI design requirements", "branch": "%s"}},
    {"type": "get_global_context", "args": {"project_id": "123", "branch": "%s"}},
    {"type": "combine", "args": {}},
    {"type": "invoke_llm", "args": {"system_prompt": "...", "user_prompt": "..."}}
]`, projectID, branch, query, branch, branch)
	systemPrompt := "You are a strategic planning assistant for GitHub project summaries."

	planJSON, err := a.InvokeLLM(prompt, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan: %v", err)
	}

	var plan []Step
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %v", err)
	}

	return plan, nil
}

func (a *GitHubAgent) ExecutePlan(plan []Step, query string, projectID string, branch string) (*AgentState, error) {
	state := &AgentState{
		OriginalQuery: query,
		Plan:          plan,
	}

	for i, step := range plan {
		log.Printf("[Step %d] Executing %s", i, step.Type)

		switch step.Type {
		case "search_by_query":
			query := step.Args["query"].(string)
			results, err := a.SearchByQuery(query, projectID, branch)
			if err != nil {
				return nil, fmt.Errorf("search step failed: %v", err)
			}

			// Store results in state
			state.SearchResults = append(state.SearchResults, SearchResult{Tasks: results})

		case "get_global_context":
			projectID := step.Args["project_id"].(string)
			gc, err := a.mongoRepo.GetGlobalContext(projectID)
			if err != nil {
				return nil, fmt.Errorf("task summary step failed: %v", err)
			}
			state.GlobalContext = gc

		case "combine":
			// Combine information from search results and task summaries
			var combined []string

			for _, result := range state.SearchResults {
				combined = append(combined, fmt.Sprintf("Result %s<br>", result.Tasks))
			}

			// Convert GlobalContext to string representation or use proper formatting
			globalContextJSON, err := json.Marshal(state.GlobalContext)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal global context: %v", err)
			}
			combined = append(combined, fmt.Sprintf("GlobalContext %s", string(globalContextJSON)))

			state.CombinedContext = "<h3>Combined Context</h3><br>" +
				strings.Join(combined, "<br><br>")

		case "invoke_llm":
			systemPrompt := step.Args["system_prompt"].(string)
			userPrompt := step.Args["user_prompt"].(string)

			// Use the stored query from previous step
			finalPrompt := fmt.Sprintf("%s<br><br>Context:<br>%s<br><br>%s",
				systemPrompt, state.CombinedContext, userPrompt)

			answer, err := a.InvokeLLM(systemPrompt, finalPrompt)
			if err != nil {
				return nil, fmt.Errorf("llm invocation failed: %v", err)
			}
			// Ensure the final answer preserves HTML formatting
			state.FinalAnswer = strings.ReplaceAll(answer, "\n", "<br>")

		default:
			log.Printf("[Step %d] Unknown step type: %s", i, step.Type)
		}
	}

	return state, nil
}
