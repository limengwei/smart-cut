package eventbus

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"smart-cut/internal/model"
)

func TestEventBus_EmitProgress(t *testing.T) {
	var mu sync.Mutex
	var received model.ProgressEvent

	bus := NewEventBus(func(name string, data interface{}) {
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, "progress", name)
		received = data.(model.ProgressEvent)
	})

	event := model.ProgressEvent{
		TaskID:   "task-1",
		Stage:    "transcribe",
		Progress: 0.5,
		Status:   model.TaskRunning,
	}

	bus.EmitProgress(event)

	assert.Equal(t, "task-1", received.TaskID)
	assert.Equal(t, "transcribe", received.Stage)
	assert.Equal(t, 0.5, received.Progress)
}

func TestEventBus_Emit(t *testing.T) {
	var receivedName string
	var receivedData interface{}

	bus := NewEventBus(func(name string, data interface{}) {
		receivedName = name
		receivedData = data
	})

	bus.Emit("custom-event", "hello")

	assert.Equal(t, "custom-event", receivedName)
	assert.Equal(t, "hello", receivedData)
}

func TestEventBus_NilEmitFunc(t *testing.T) {
	// 不注入 emitFunc，不应 panic
	bus := NewEventBus(nil)
	bus.EmitProgress(model.ProgressEvent{TaskID: "test"})
	bus.Emit("anything", nil)
}

func TestEventBus_SetEmitFunc(t *testing.T) {
	bus := NewEventBus(nil)

	var received bool
	bus.SetEmitFunc(func(name string, data interface{}) {
		received = true
	})

	bus.Emit("test", nil)
	assert.True(t, received)
}
