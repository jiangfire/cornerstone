package asyncworker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPool_SubmitRunsTaskAndStopWaits(t *testing.T) {
	p := New(context.Background())
	var counter atomic.Int32
	done := make(chan struct{})

	require.NoError(t, p.Submit("inc", func(ctx context.Context) {
		counter.Add(1)
		close(done)
	}))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("submitted task did not run")
	}

	require.NoError(t, p.Stop(2*time.Second))
	require.Equal(t, int32(1), counter.Load())
}

func TestPool_PanicDoesNotCrashProcess(t *testing.T) {
	p := New(context.Background())
	completed := make(chan struct{})

	require.NoError(t, p.Submit("panic-task", func(ctx context.Context) {
		defer close(completed)
		panic("intentional test panic")
	}))

	select {
	case <-completed:
	case <-time.After(2 * time.Second):
		t.Fatal("panicked task did not exit")
	}

	survivor := make(chan struct{})
	require.NoError(t, p.Submit("after-panic", func(ctx context.Context) {
		close(survivor)
	}))
	select {
	case <-survivor:
	case <-time.After(2 * time.Second):
		t.Fatal("pool refused new task after a peer panicked")
	}

	require.NoError(t, p.Stop(2*time.Second))
}

func TestPool_StopCancelsContextForRunningTasks(t *testing.T) {
	p := New(context.Background())
	observed := make(chan struct{})

	require.NoError(t, p.Submit("long-runner", func(ctx context.Context) {
		<-ctx.Done()
		close(observed)
	}))

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- p.Stop(2 * time.Second)
	}()

	select {
	case <-observed:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not observe pool ctx cancellation")
	}

	select {
	case err := <-stopDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after task exited")
	}
}

func TestPool_SubmitAfterStopReturnsError(t *testing.T) {
	p := New(context.Background())
	require.NoError(t, p.Stop(time.Second))

	err := p.Submit("late", func(ctx context.Context) {})
	require.ErrorIs(t, err, ErrPoolClosed)
}
