package eventbus

import (
	"sync"

	"smart-cut/internal/model"
)

// EventBus 封装事件推送能力
// 在 Wails 环境下，EmitFunc 调用 app.Events.Emit
// 在测试环境下，可注入 mock
type EventBus struct {
	mu       sync.RWMutex
	emitFunc func(eventName string, data interface{})
}

// NewEventBus 创建 EventBus
// emitFunc: 实际推送函数（Wails 里是 app.Events.Emit 的包装）
func NewEventBus(emitFunc func(string, interface{})) *EventBus {
	return &EventBus{emitFunc: emitFunc}
}

// EmitProgress 推送进度事件
func (b *EventBus) EmitProgress(event model.ProgressEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.emitFunc != nil {
		b.emitFunc("progress", event)
	}
}

// Emit 推送任意事件
func (b *EventBus) Emit(eventName string, data interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.emitFunc != nil {
		b.emitFunc(eventName, data)
	}
}

// SetEmitFunc 替换推送函数（用于运行时注入 Wails app）
func (b *EventBus) SetEmitFunc(emitFunc func(string, interface{})) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.emitFunc = emitFunc
}
