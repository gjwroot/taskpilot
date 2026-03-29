package model

type Task struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`   // todo, doing, done
	Priority    int    `json:"priority"` // 0=P0(紧急) 1=P1 2=P2 3=P3
	DueDate     string `json:"dueDate"`  // ISO date string, empty if not set
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	Tags        string `json:"tags"`      // 逗号分隔标签
}
