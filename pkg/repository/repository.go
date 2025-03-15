package repository

type GlobalContextRepository struct {
	// Placeholder for MongoDB integration
}

type GlobalContext struct {
	ProjectID string
	Data      interface{}
}

var GR *GlobalContextRepository

// GetGlobalContext retrieves the global context for a given project
func (r *GlobalContextRepository) GetGlobalContext(projectID string) (*GlobalContext, error) {
	return &GlobalContext{
		ProjectID: projectID,
		Data:      nil,
	}, nil
}
