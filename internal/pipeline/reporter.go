package pipeline

import (
	"smart-cut/internal/eventbus"
	"smart-cut/internal/model"
)

// eventBusReporter 是 ProgressReporter 的实现，通过 EventBus 推送进度
type eventBusReporter struct {
	bus    *eventbus.EventBus
	taskID string
}

// NewEventBusReporter 创建基于 EventBus 的 ProgressReporter
func NewEventBusReporter(bus *eventbus.EventBus, taskID string) ProgressReporter {
	return &eventBusReporter{bus: bus, taskID: taskID}
}

func (r *eventBusReporter) Report(stage, step string, progress float64) {
	r.bus.EmitProgress(model.ProgressEvent{
		TaskID:   r.taskID,
		Stage:    stage,
		Step:     step,
		Progress: progress,
		Status:   model.TaskRunning,
	})
}

func (r *eventBusReporter) Error(stage string, err error) {
	r.bus.EmitProgress(model.ProgressEvent{
		TaskID: r.taskID,
		Stage:  stage,
		Status: model.TaskError,
		Error:  err.Error(),
	})
}

func (r *eventBusReporter) Done(stage string, payload interface{}) {
	r.bus.EmitProgress(model.ProgressEvent{
		TaskID:  r.taskID,
		Stage:   stage,
		Status:  model.TaskDone,
		Payload: payload,
	})
}

// noopReporter 是 ProgressReporter 的空实现（用于测试）
type noopReporter struct{}

// NewNoopReporter 创建空 ProgressReporter
func NewNoopReporter() ProgressReporter {
	return &noopReporter{}
}

func (r *noopReporter) Report(stage, step string, progress float64) {}
func (r *noopReporter) Error(stage string, err error)               {}
func (r *noopReporter) Done(stage string, payload interface{})      {}
