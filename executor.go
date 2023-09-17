package failsafe

import (
	"context"
	"math"
	"sync"

	"github.com/failsafe-go/failsafe-go/common"
)

// Run executes the fn, with failures being handled by the policies, until successful or until the policies are exceeded.
func Run(fn func() error, policies ...Policy[any]) error {
	return NewExecutor[any](policies...).Run(fn)
}

// RunWithExecution executes the fn, with failures being handled by the policies, until successful or until the policies
// are exceeded.
func RunWithExecution(fn func(exec Execution[any]) error, policies ...Policy[any]) error {
	return NewExecutor[any](policies...).RunWithExecution(fn)
}

// Get executes the fn, with failures being handled by the policies, until a successful result is returned or the
// policies are exceeded.
func Get[R any](fn func() (R, error), policies ...Policy[R]) (R, error) {
	return NewExecutor[R](policies...).Get(fn)
}

// GetWithExecution executes the fn, with failures being handled by the policies, until a successful result is returned
// or the policies are exceeded.
func GetWithExecution[R any](fn func(exec Execution[R]) (R, error), policies ...Policy[R]) (R, error) {
	return NewExecutor[R](policies...).GetWithExecution(fn)
}

// RunAsync executes the fn in a goroutine, with failures being handled by the policies, until successful or until the
// policies are exceeded.
func RunAsync(fn func() error, policies ...Policy[any]) ExecutionResult[any] {
	return NewExecutor[any](policies...).RunAsync(fn)
}

// RunWithExecutionAsync executes the fn in a goroutine, with failures being handled by the policies, until successful or
// until the policies are exceeded.
func RunWithExecutionAsync(fn func(exec Execution[any]) error, policies ...Policy[any]) ExecutionResult[any] {
	return NewExecutor[any](policies...).RunWithExecutionAsync(fn)
}

// GetAsync executes the fn in a goroutine, with failures being handled by the policies, until a successful result is returned
// or the policies are exceeded.
func GetAsync[R any](fn func() (R, error), policies ...Policy[R]) ExecutionResult[R] {
	return NewExecutor[R](policies...).GetAsync(fn)
}

// GetWithExecutionAsync executes the fn in a goroutine, with failures being handled by the policies, until a successful
// result is returned or the policies are exceeded.
func GetWithExecutionAsync[R any](fn func(exec Execution[R]) (R, error), policies ...Policy[R]) ExecutionResult[R] {
	return NewExecutor[R](policies...).GetWithExecutionAsync(fn)
}

// Executor handles failures according to configured policies. See [NewExecutor] for details.
//
// This type is concurrency safe.
type Executor[R any] interface {
	// WithContext returns a new copy of the Executor with the ctx configured. Any executions created with the resulting
	// Executor will be canceled when the ctx is done. Executions can cooperate with cancellation by checking
	// Execution.Canceled or Execution.IsCanceled.
	WithContext(ctx context.Context) Executor[R]

	// OnComplete registers the listener to be called when an execution is complete.
	OnComplete(listener func(ExecutionCompletedEvent[R])) Executor[R]

	// OnSuccess registers the listener to be called when an execution is successful. If multiple policies, are configured,
	// this handler is called when execution is complete and all policies succeed. If all policies do not succeed, then the
	// OnFailure registered listener is called instead.
	OnSuccess(listener func(ExecutionCompletedEvent[R])) Executor[R]

	// OnFailure registers the listener to be called when an execution fails. This occurs when the execution fails according
	// to some policy, and all policies have been exceeded.
	OnFailure(listener func(ExecutionCompletedEvent[R])) Executor[R]

	// Run executes the fn until successful or until the configured policies are exceeded.
	//
	// Any panic causes the execution to stop immediately without calling any event listeners.
	Run(fn func() error) error

	// RunWithExecution executes the fn until successful or until the configured policies are exceeded, while providing an
	// Execution to the fn.
	//
	// Any panic causes the execution to stop immediately without calling any event listeners.
	RunWithExecution(fn func(exec Execution[R]) error) error

	// Get executes the fn until a successful result is returned or the configured policies are exceeded.
	//
	// Any panic causes the execution to stop immediately without calling any event listeners.
	Get(fn func() (R, error)) (R, error)

	// GetWithExecution executes the fn until a successful result is returned or the configured policies are exceeded, while
	// providing an Execution to the fn.
	//
	// Any panic causes the execution to stop immediately without calling any event listeners.
	GetWithExecution(fn func(exec Execution[R]) (R, error)) (R, error)

	// RunAsync executes the fn in a goroutine until successful or until the configured policies are exceeded.
	//
	// Any panic causes the execution to stop immediately without calling any event listeners.
	RunAsync(fn func() error) ExecutionResult[R]

	// RunWithExecutionAsync executes the fn in a goroutine until successful or until the configured policies are exceeded,
	// while providing an Execution to the fn.
	//
	// Any panic causes the execution to stop immediately without calling any event listeners.
	RunWithExecutionAsync(fn func(exec Execution[R]) error) ExecutionResult[R]

	// GetAsync executes the fn in a goroutine until a successful result is returned or the configured policies are exceeded.
	//
	// Any panic causes the execution to stop immediately without calling any event listeners.
	GetAsync(fn func() (R, error)) ExecutionResult[R]

	// GetWithExecutionAsync executes the fn in a goroutine until a successful result is returned or the configured policies
	// are exceeded, while providing an Execution to the fn.
	//
	// Any panic causes the execution to stop immediately without calling any event listeners.
	GetWithExecutionAsync(fn func(exec Execution[R]) (R, error)) ExecutionResult[R]
}

type executor[R any] struct {
	policies   []Policy[R]
	ctx        context.Context
	onComplete func(ExecutionCompletedEvent[R])
	onSuccess  func(ExecutionCompletedEvent[R])
	onFailure  func(ExecutionCompletedEvent[R])
}

// NewExecutor creates and returns a new Executor for result type R that will handle failures according to the given
// policies. The policies are composed around a func and will handle its results in reverse order. For example, consider:
//
//	failsafe.NewExecutor(fallback, retryPolicy, circuitBreaker).Get(fn)
//
// This creates the following composition when executing a func and handling its result:
//
//	Fallback(RetryPolicy(CircuitBreaker(func)))
func NewExecutor[R any](policies ...Policy[R]) Executor[R] {
	return &executor[R]{
		policies: policies,
	}
}

func (e *executor[R]) WithContext(ctx context.Context) Executor[R] {
	c := *e
	c.ctx = ctx
	return &c
}

func (e *executor[R]) OnComplete(listener func(ExecutionCompletedEvent[R])) Executor[R] {
	e.onComplete = listener
	return e
}

func (e *executor[R]) OnSuccess(listener func(ExecutionCompletedEvent[R])) Executor[R] {
	e.onSuccess = listener
	return e
}

func (e *executor[R]) OnFailure(listener func(ExecutionCompletedEvent[R])) Executor[R] {
	e.onFailure = listener
	return e
}

func (e *executor[R]) Run(fn func() error) error {
	_, err := e.executeSync(func(_ Execution[R]) (R, error) {
		return *(new(R)), fn()
	})
	return err
}

func (e *executor[R]) RunWithExecution(fn func(exec Execution[R]) error) error {
	_, err := e.executeSync(func(exec Execution[R]) (R, error) {
		return *(new(R)), fn(exec)
	})
	return err
}

func (e *executor[R]) Get(fn func() (R, error)) (R, error) {
	return e.executeSync(func(_ Execution[R]) (R, error) {
		return fn()
	})
}

func (e *executor[R]) GetWithExecution(fn func(exec Execution[R]) (R, error)) (R, error) {
	return e.executeSync(func(exec Execution[R]) (R, error) {
		return fn(exec)
	})
}

func (e *executor[R]) RunAsync(fn func() error) ExecutionResult[R] {
	return e.executeAsync(func(_ Execution[R]) (R, error) {
		return *(new(R)), fn()
	})
}

func (e *executor[R]) RunWithExecutionAsync(fn func(exec Execution[R]) error) ExecutionResult[R] {
	return e.executeAsync(func(exec Execution[R]) (R, error) {
		return *(new(R)), fn(exec)
	})
}

func (e *executor[R]) GetAsync(fn func() (R, error)) ExecutionResult[R] {
	return e.executeAsync(func(_ Execution[R]) (R, error) {
		return fn()
	})
}

func (e *executor[R]) GetWithExecutionAsync(fn func(exec Execution[R]) (R, error)) ExecutionResult[R] {
	return e.executeAsync(func(exec Execution[R]) (R, error) {
		return fn(exec)
	})
}

// This type mirrors part of policy.ExecutionInternal, which we don't import here to avoid a cycle.
type executionInternal[R any] interface {
	Record(result *common.PolicyResult[R]) *common.PolicyResult[R]
}

// This type mirrors part of policy.Executor, which we don't import here to avoid a cycle.
type policyExecutor[R any] interface {
	Apply(innerFn func(Execution[R]) *common.PolicyResult[R]) func(Execution[R]) *common.PolicyResult[R]
}

func (e *executor[R]) executeSync(fn func(exec Execution[R]) (R, error)) (R, error) {
	er := e.execute(fn)
	return er.Result, er.Error
}

func (e *executor[R]) executeAsync(fn func(exec Execution[R]) (R, error)) ExecutionResult[R] {
	result := &executionResult[R]{
		doneChan: make(chan any, 1),
	}
	go func() {
		result.complete(e.execute(fn))
	}()
	return result
}

func (e *executor[R]) execute(fn func(exec Execution[R]) (R, error)) *common.PolicyResult[R] {
	outerFn := func(exec Execution[R]) *common.PolicyResult[R] {
		// Copy exec before passing to user provided func
		execCopy := *(exec.(*execution[R]))
		result, err := fn(&execCopy)
		er := &common.PolicyResult[R]{
			Result:     result,
			Error:      err,
			Complete:   true,
			Success:    true,
			SuccessAll: true,
		}
		execInternal := exec.(executionInternal[R])
		r := execInternal.Record(er)
		return r
	}

	// Compose policy executors from the innermost policy to the outermost
	for i, policyIndex := len(e.policies)-1, 0; i >= 0; i, policyIndex = i-1, policyIndex+1 {
		pe := e.policies[i].ToExecutor(policyIndex, *(new(R))).(policyExecutor[R])
		outerFn = pe.Apply(outerFn)
	}

	// Prepare execution
	canceledIndex := -1
	exec := &execution[R]{
		mtx:           &sync.Mutex{},
		canceled:      make(chan any),
		canceledIndex: &canceledIndex,
		ctx:           e.ctx,
	}

	// Propagate context cancellations to the execution
	ctx := e.ctx
	var stopAfterFunc func() bool
	if ctx != nil {
		stopAfterFunc = context.AfterFunc(ctx, func() {
			exec.Cancel(math.MaxInt, &common.PolicyResult[R]{
				Error:    ctx.Err(),
				Complete: true,
			})
		})
	}

	// Initialize first attempt and execute
	exec.InitializeAttempt(canceledIndex)
	er := outerFn(exec)

	// Stop the Context AfterFunc and call listeners
	if stopAfterFunc != nil {
		stopAfterFunc()
	}
	if e.onSuccess != nil && er.SuccessAll {
		e.onSuccess(newExecutionCompletedEvent(er, exec))
	} else if e.onFailure != nil && !er.SuccessAll {
		e.onFailure(newExecutionCompletedEvent(er, exec))
	}
	if e.onComplete != nil {
		e.onComplete(newExecutionCompletedEvent(er, exec))
	}
	return er
}
