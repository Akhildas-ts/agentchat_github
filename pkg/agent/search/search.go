package search

import (
	"agentchat/pkg/usecase"
	"context"
	"fmt"
)

// Search functionality extracted from the agent
func SearchInRepository(query, projectID, branch string) ([]string, error) {
	ctx := context.Background()

	// Use the same usecase as in the agent
	descriptions, err := usecase.GUC.SearchByQuestion(ctx, query, projectID, branch)
	if err != nil {
		return nil, fmt.Errorf("search failed: %v", err)
	}

	return descriptions, nil
}

// Advanced search functions can be added here
// For example, searching with filters, pagination, etc.
