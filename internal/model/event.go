package model

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskRunning TaskStatus = "running"
	TaskDone    TaskStatus = "done"
	TaskError   TaskStatus = "error"
)

// ProgressEvent 进度事件（通过 EventBus/Wails Event 推送到前端）
type ProgressEvent struct {
	TaskID   string      `json:"taskId"`
	Stage    string      `json:"stage"`    // transcribe/analyze/edit/export/subtitle
	Step     string      `json:"step"`     // 当前步骤描述
	Progress float64     `json:"progress"` // 0-1
	Status   TaskStatus  `json:"status"`
	Error    string      `json:"error,omitempty"`
	Payload  interface{} `json:"payload,omitempty"`
}
