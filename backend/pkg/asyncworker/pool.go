// Package asyncworker provides a lightweight goroutine pool for fire-and-forget
// background tasks (e.g. plugin triggers, webhook fan-out). It is intentionally
// minimal: no queueing, no rate limiting — just structured lifecycle, panic
// recovery, and graceful shutdown.
package asyncworker

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Pool 异步任务池。
//
// 设计要点：
//   - 每个任务在独立 goroutine 中执行，遵循 fire-and-forget 语义；
//   - panic 通过 recover 捕获并打到日志，不会污染主进程；
//   - Stop 取消上下文并等待所有任务结束（带超时），避免关库后仍有任务持有 DB 连接。
type Pool struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	closed atomic.Bool
}

// New 创建一个新池子。parent 可以为 nil（默认使用 context.Background）。
func New(parent context.Context) *Pool {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Pool{ctx: ctx, cancel: cancel}
}

// Context 返回池子的内部 context，task 可以监听其取消信号实现协作式停止。
func (p *Pool) Context() context.Context {
	return p.ctx
}

// ErrPoolClosed 在池子已经 Stop 之后提交任务时返回。
var ErrPoolClosed = errors.New("async worker pool already stopped")

// Submit 提交一个 fire-and-forget 任务。
//
// name 仅用于日志归因（panic 时记录），不参与调度。
// fn 内部应自行处理错误；不会有任何 channel 把错误传回 caller。
//
// 注意：返回 nil 表示已成功登记；并不代表任务已经开始执行。
func (p *Pool) Submit(name string, fn func(ctx context.Context)) error {
	if p.closed.Load() {
		return ErrPoolClosed
	}
	p.wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				zap.L().Error("async worker task panic recovered",
					zap.String("task", name),
					zap.Any("panic", r),
					zap.ByteString("stack", debug.Stack()),
				)
			}
		}()
		fn(p.ctx)
	})
	return nil
}

// Stop 停止池子：取消内部 ctx 并等待已经在跑的任务结束，但最多等 timeout。
// 超时返回 context.DeadlineExceeded；正常返回 nil。
// 多次调用安全（后续调用直接返回 nil）。
func (p *Pool) Stop(timeout time.Duration) error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	p.cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.wg.Wait()
	}()

	if timeout <= 0 {
		<-done
		return nil
	}
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}
