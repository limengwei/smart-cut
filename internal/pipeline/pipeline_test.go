package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStep struct {
	name string
	fn   func(ctx *Context, reporter ProgressReporter) error
}

func (s *mockStep) Name() string { return s.name }
func (s *mockStep) Run(ctx *Context, reporter ProgressReporter) error {
	if s.fn != nil {
		return s.fn(ctx, reporter)
	}
	return nil
}

type mockReporter struct {
	reports []reportEntry
	errors  []errorEntry
	dones   []doneEntry
}

type reportEntry struct {
	stage, step string
	progress    float64
}
type errorEntry struct {
	stage string
	err   error
}
type doneEntry struct {
	stage   string
	payload interface{}
}

func (r *mockReporter) Report(stage, step string, progress float64) {
	r.reports = append(r.reports, reportEntry{stage, step, progress})
}
func (r *mockReporter) Error(stage string, err error) {
	r.errors = append(r.errors, errorEntry{stage, err})
}
func (r *mockReporter) Done(stage string, payload interface{}) {
	r.dones = append(r.dones, doneEntry{stage, payload})
}

func TestPipeline_Execute_AllStepsSucceed(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	executed := []string{}
	p.AddStep(&mockStep{name: "step1", fn: func(ctx *Context, r ProgressReporter) error {
		executed = append(executed, "step1")
		return nil
	}})
	p.AddStep(&mockStep{name: "step2", fn: func(ctx *Context, r ProgressReporter) error {
		executed = append(executed, "step2")
		return nil
	}})

	ctx := &Context{Cancel: context.Background()}
	err := p.Execute(ctx)

	require.NoError(t, err)
	assert.Equal(t, []string{"step1", "step2"}, executed)
	assert.Len(t, reporter.reports, 2)
	assert.Len(t, reporter.dones, 1)
	assert.Equal(t, "pipeline", reporter.dones[0].stage)
	assert.Empty(t, reporter.errors)
}

func TestPipeline_Execute_StepFails(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	stepErr := errors.New("boom")
	p.AddStep(&mockStep{name: "failing", fn: func(ctx *Context, r ProgressReporter) error {
		return stepErr
	}})
	p.AddStep(&mockStep{name: "never-run", fn: func(ctx *Context, r ProgressReporter) error {
		t.Fatal("should not run after failure")
		return nil
	}})

	ctx := &Context{Cancel: context.Background()}
	err := p.Execute(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failing")
	assert.Contains(t, err.Error(), "boom")
	assert.Len(t, reporter.errors, 1)
	assert.Equal(t, "failing", reporter.errors[0].stage)
}

func TestPipeline_Execute_Cancelled(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	p.AddStep(&mockStep{name: "step1", fn: func(ctx *Context, r ProgressReporter) error {
		t.Fatal("should not run when cancelled")
		return nil
	}})

	ctx := &Context{Cancel: cancelCtx}
	err := p.Execute(ctx)

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestPipeline_Execute_EmptyPipeline(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	ctx := &Context{Cancel: context.Background()}
	err := p.Execute(ctx)

	require.NoError(t, err)
	assert.Len(t, reporter.dones, 1)
}

func TestPipeline_ProgressCalculation(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	p.AddStep(&mockStep{name: "a"})
	p.AddStep(&mockStep{name: "b"})
	p.AddStep(&mockStep{name: "c"})

	ctx := &Context{Cancel: context.Background()}
	_ = p.Execute(ctx)

	assert.Len(t, reporter.reports, 3)
	assert.Equal(t, 0.0, reporter.reports[0].progress)
	assert.InDelta(t, 0.333, reporter.reports[1].progress, 0.01)
	assert.InDelta(t, 0.666, reporter.reports[2].progress, 0.01)
}

func TestNoopReporter_DoesNotPanic(t *testing.T) {
	r := NewNoopReporter()
	r.Report("a", "b", 0.5)
	r.Error("a", errors.New("test"))
	r.Done("a", "payload")
}
