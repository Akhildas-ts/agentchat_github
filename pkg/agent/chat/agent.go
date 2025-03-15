package chat

import (
	"context"
	"fmt"

	"agentchat/pkg/anthropic"
	"agentchat/pkg/config"
	pineconedb "agentchat/pkg/db/pineconeDB"
	mongorepo "agentchat/pkg/repository"
	"agentchat/pkg/usecase"
	"agentchat/pkg/utils/constants"

	"github.com/pinecone-io/go-pinecone/pinecone"
)

type GitHubAgent struct {
	mongoRepo      *mongorepo.GlobalContextRepository
	pineconeClient *pinecone.Client
	config         *config.Config
	tools          map[string]interface{}
	usecase        *usecase.GitHubUseCase
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
