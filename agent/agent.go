package agent

import (
	"agentchat/pkg/anthropic"
	"agentchat/pkg/config"
	pineconedb "agentchat/pkg/db/pineconeDB"
	mongorepo "agentchat/pkg/repository"
	"agentchat/pkg/usecase"
	"agentchat/pkg/utils/constants"
	"context"
	"time"

	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/pinecone-io/go-pinecone/pinecone"
)

type SearchByQueryFunc func(query string) ([]string, error)
type GetTaskSummaryFunc func(projectID string) ([]string, error)
type InvokeLLMFunc func(systemPrompt, userPrompt string) (string, error)
type GitHubAgent struct {
	mongoRepo      *mongorepo.GlobalContextRepository
	pineconeClient *pinecone.Client
	config         *config.Config
	tools          map[string]interface{}
	usecase        *usecase.GitHubUseCase
}

type SearchResult struct {
	Tasks []string
}

type AgentState struct {
	OriginalQuery   string                   `json:"original_query"`
	Plan            []Step                   `json:"plan"`
	SearchResults   []SearchResult           `json:"search_results"`
	GlobalContext   *mongorepo.GlobalContext `json:"global_context"`
	CombinedContext string                   `json:"combined_context"`
	FinalAnswer     string                   `json:"final_answer"`
}

type Step struct {
	Type string                 `json:"type"`
	Args map[string]interface{} `json:"args"`
}

func NewGitHubAgent(mongoRepo *mongorepo.GlobalContextRepository, pineconeClient *pinecone.Client, usecase *usecase.GitHubUseCase) *GitHubAgent {
	agent := &GitHubAgent{
		mongoRepo:      mongoRepo,
		pineconeClient: pineconeClient,
		config:         config.Cfg,
		usecase:        usecase,
		tools:          make(map[string]interface{}),
	}

	// Register tool functions
	agent.tools["search_by_query"] = agent.SearchByQuery
	// agent.tools["get_global_context"] = agent.GetTaskSummary
	agent.tools["invoke_llm"] = agent.InvokeLLM

	return agent
}

func SearchByAgent(query, projectID, branch string) (string, error) {

	tokenModel, err := usecase.GUC.GetToken(projectID)
	if err != nil {
		return "", err
	}

	if tokenModel.IsExpired {
		return "", constants.ErrTokenExpired
	}

	// Init mongo,pinecone client
	mongoClient := mongorepo.GR
	pineconeClient := pineconedb.PC
	githubUsecase := usecase.GUC
	agent := NewGitHubAgent(mongoClient, pineconeClient, githubUsecase)
	return agent.ProcessQuery(query, projectID, branch)

}

func (a *GitHubAgent) SearchByQuery(query, projectID, branch string) ([]string, error) {
	ctx := context.Background()

	descriptions, err := a.usecase.SearchByQuestion(ctx, query, projectID, branch)

	if err != nil {
		fmt.Println("HHH")
		return nil, err
	}

	return descriptions, nil
}

func (a *GitHubAgent) InvokeLLM(userPrompt string, systemPrompt string) (string, error) {

	fmt.Println("from INvokeLLm")

	fmt.Println("a apikey", a.config.ANTHROPIC_API_KEY)
	answer, err := anthropic.GetAnswerFromAnthropic(userPrompt, a.config.ANTHROPIC_API_KEY, systemPrompt)
	if err != nil {

		return "", err
	}
	fmt.Println("from INvokeLLm2")
	return answer, nil
}

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

			// Combine search results into a string for LLM

			// Store the query for later use

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

			combined = append(combined, fmt.Sprintf("GlobalContext %s ", state.GlobalContext))

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
