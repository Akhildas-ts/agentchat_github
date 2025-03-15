package chat

import "agentchat/pkg/repository"

// Add GlobalContext type alias for convenience
type GlobalContext = repository.GlobalContext

type SearchByQueryFunc func(query string) ([]string, error)
type GetTaskSummaryFunc func(projectID string) ([]string, error)
type InvokeLLMFunc func(systemPrompt, userPrompt string) (string, error)

type SearchResult struct {
	Tasks []string
}

type AgentState struct {
	OriginalQuery   string         `json:"original_query"`
	Plan            []Step         `json:"plan"`
	SearchResults   []SearchResult `json:"search_results"`
	GlobalContext   *GlobalContext `json:"global_context"`
	CombinedContext string         `json:"combined_context"`
	FinalAnswer     string         `json:"final_answer"`
}

type Step struct {
	Type string                 `json:"type"`
	Args map[string]interface{} `json:"args"`
}

type State struct {
	GlobalContext interface{}
	// Add other state fields as needed
}
