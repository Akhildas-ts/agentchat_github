package usecase

import "context"

type GitHubUseCase struct {
    // Add fields as needed
}

var GUC *GitHubUseCase

func (g *GitHubUseCase) GetToken(projectID string) (*TokenModel, error) {
    return &TokenModel{}, nil
}

func (g *GitHubUseCase) SearchByQuestion(ctx context.Context, query, projectID, branch string) ([]string, error) {
    return []string{}, nil
}

type TokenModel struct {
    IsExpired bool
} 