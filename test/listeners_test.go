package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"failsafe"
	"failsafe/circuitbreaker"
	"failsafe/fallback"
	"failsafe/internal/testutil"
	"failsafe/retrypolicy"
)

// Asserts that listeners are called the expected number of times for a successful completion.
func TestListenersOnSuccess(t *testing.T) {
	// Given - Fail 4 times then succeed
	stub := testutil.ErrorNTimesThenReturn[bool](testutil.InvalidStateError{}, 2, false, false, true)
	rpBuilder := retrypolicy.Builder[bool]().HandleResult(false).WithMaxAttempts(10)
	cbBuilder := circuitbreaker.Builder[bool]().HandleResult(false).WithDelay(0)
	fbBuilder := fallback.BuilderOfResult(false)
	stats := &listenerStats{}
	registerRpListeners(stats, rpBuilder)
	registerCbListeners(stats, cbBuilder)
	registerFbListeners(stats, fbBuilder)
	executor := failsafe.With[bool](fbBuilder.Build(), rpBuilder.Build(), cbBuilder.Build())
	registerExecutorListeners(stats, executor)

	// When
	executor.GetWithExecution(stub)

	// Then
	assert.Equal(t, 0, stats.abort)
	assert.Equal(t, 4, stats.rpFailedAttempt)
	assert.Equal(t, 0, stats.retriesExceeded)
	assert.Equal(t, 4, stats.retryScheduled)
	assert.Equal(t, 4, stats.retry)
	assert.Equal(t, 1, stats.rpSuccess)
	assert.Equal(t, 0, stats.rpFailure)

	assert.Equal(t, 4, stats.open)
	assert.Equal(t, 4, stats.halfOpen)
	assert.Equal(t, 1, stats.close)
	assert.Equal(t, 1, stats.cbSuccess)
	assert.Equal(t, 4, stats.cbFailure)

	assert.Equal(t, 0, stats.fbFailedAttempt)
	assert.Equal(t, 1, stats.fbSuccess)
	assert.Equal(t, 0, stats.fbFailure)

	assert.Equal(t, 1, stats.complete)
	assert.Equal(t, 1, stats.success)
	assert.Equal(t, 0, stats.failure)
}

// Asserts that listeners are called the expected number of times for an unhandled failure.
func TestListenersForUnhandledFailure(t *testing.T) {
	// Given - Fail 2 times then don't match policy
	stub := testutil.ErrorNTimesThenError[bool](testutil.InvalidStateError{}, 2, testutil.InvalidArgumentError{})
	rpBuilder := retrypolicy.Builder[bool]().HandleErrors(testutil.InvalidStateError{}).WithMaxAttempts(10)
	cbBuilder := circuitbreaker.Builder[bool]().WithDelay(0)
	stats := &listenerStats{}
	registerRpListeners(stats, rpBuilder)
	registerCbListeners(stats, cbBuilder)
	executor := failsafe.With[bool](rpBuilder.Build(), cbBuilder.Build())
	registerExecutorListeners(stats, executor)

	// When
	executor.GetWithExecution(stub)

	// Then
	assert.Equal(t, 0, stats.abort)
	assert.Equal(t, 2, stats.rpFailedAttempt)
	assert.Equal(t, 0, stats.retriesExceeded)
	assert.Equal(t, 2, stats.retryScheduled)
	assert.Equal(t, 2, stats.retry)
	assert.Equal(t, 1, stats.rpSuccess)
	assert.Equal(t, 0, stats.rpFailure)

	assert.Equal(t, 3, stats.open)
	assert.Equal(t, 2, stats.halfOpen)
	assert.Equal(t, 0, stats.close)
	assert.Equal(t, 0, stats.cbSuccess)
	assert.Equal(t, 3, stats.cbFailure)

	assert.Equal(t, 1, stats.complete)
	assert.Equal(t, 0, stats.success)
	assert.Equal(t, 1, stats.failure)
}

// Asserts that listeners are called the expected number of times when retries are exceeded.
func TestListenersForRetriesExceeded(t *testing.T) {
	// Given - Fail 4 times and exceed retries
	stub := testutil.ErrorNTimesThenReturn[bool](testutil.InvalidStateError{}, 10)
	rpBuilder := retrypolicy.Builder[bool]().WithMaxRetries(3)
	cbBuilder := circuitbreaker.Builder[bool]().WithDelay(0)
	stats := &listenerStats{}
	registerRpListeners(stats, rpBuilder)
	registerCbListeners(stats, cbBuilder)
	executor := failsafe.With[bool](rpBuilder.Build(), cbBuilder.Build())
	registerExecutorListeners(stats, executor)

	// When
	executor.GetWithExecution(stub)

	// Then
	assert.Equal(t, 0, stats.abort)
	assert.Equal(t, 4, stats.rpFailedAttempt)
	assert.Equal(t, 1, stats.retriesExceeded)
	assert.Equal(t, 3, stats.retryScheduled)
	assert.Equal(t, 3, stats.retry)
	assert.Equal(t, 0, stats.rpSuccess)
	assert.Equal(t, 1, stats.rpFailure)

	assert.Equal(t, 4, stats.open)
	assert.Equal(t, 3, stats.halfOpen)
	assert.Equal(t, 0, stats.close)
	assert.Equal(t, 0, stats.cbSuccess)
	assert.Equal(t, 4, stats.cbFailure)

	assert.Equal(t, 1, stats.complete)
	assert.Equal(t, 0, stats.success)
	assert.Equal(t, 1, stats.failure)
}

func TestListenersForAbort(t *testing.T) {
	// Given - Fail twice then abort
	stub := testutil.ErrorNTimesThenError[bool](testutil.InvalidStateError{}, 3, testutil.InvalidArgumentError{})
	rpBuilder := retrypolicy.Builder[bool]().AbortOnErrors(testutil.InvalidArgumentError{}).WithMaxRetries(3)
	cbBuilder := circuitbreaker.Builder[bool]().WithDelay(0)
	stats := &listenerStats{}
	registerRpListeners(stats, rpBuilder)
	registerCbListeners(stats, cbBuilder)
	executor := failsafe.With[bool](rpBuilder.Build(), cbBuilder.Build())
	registerExecutorListeners(stats, executor)

	// When
	executor.GetWithExecution(stub)

	// Then
	assert.Equal(t, 1, stats.abort)
	assert.Equal(t, 4, stats.rpFailedAttempt)
	assert.Equal(t, 0, stats.retriesExceeded)
	assert.Equal(t, 3, stats.retryScheduled)
	assert.Equal(t, 3, stats.retry)
	assert.Equal(t, 0, stats.rpSuccess)
	assert.Equal(t, 1, stats.rpFailure)

	assert.Equal(t, 4, stats.open)
	assert.Equal(t, 3, stats.halfOpen)
	assert.Equal(t, 0, stats.close)
	assert.Equal(t, 0, stats.cbSuccess)
	assert.Equal(t, 4, stats.cbFailure)

	assert.Equal(t, 1, stats.complete)
	assert.Equal(t, 0, stats.success)
	assert.Equal(t, 1, stats.failure)
}

func TestListenersForFailingRetryPolicy(t *testing.T) {
	// Given - Fail 10 times
	stub := testutil.ErrorNTimesThenReturn[bool](testutil.InvalidStateError{}, 10)
	// With failing RetryPolicy
	rpBuilder := retrypolicy.Builder[bool]()
	// And successful CircuitBreaker and Fallback
	cbBuilder := circuitbreaker.Builder[bool]().HandleErrors(testutil.InvalidArgumentError{}).WithDelay(0)
	fbBuilder := fallback.BuilderOfResult[bool](true).HandleErrors(testutil.InvalidArgumentError{})
	stats := &listenerStats{}
	registerRpListeners(stats, rpBuilder)
	registerCbListeners(stats, cbBuilder)
	registerFbListeners(stats, fbBuilder)
	executor := failsafe.With[bool](fbBuilder.Build(), rpBuilder.Build(), cbBuilder.Build())
	registerExecutorListeners(stats, executor)

	// When
	executor.GetWithExecution(stub)

	// Then
	assert.Equal(t, 0, stats.rpSuccess)
	assert.Equal(t, 1, stats.rpFailure)

	assert.Equal(t, 3, stats.cbSuccess)
	assert.Equal(t, 0, stats.cbFailure)

	assert.Equal(t, 0, stats.fbFailedAttempt)
	assert.Equal(t, 1, stats.fbSuccess)
	assert.Equal(t, 0, stats.fbFailure)

	assert.Equal(t, 1, stats.complete)
	assert.Equal(t, 0, stats.success)
	assert.Equal(t, 1, stats.failure)
}

func TestListenersForFailingCircuitBreaker(t *testing.T) {
	// Given - Fail 10 times
	stub := testutil.ErrorNTimesThenReturn[bool](testutil.InvalidStateError{}, 10)
	// With successful RetryPolicy
	rpBuilder := retrypolicy.Builder[bool]().HandleErrors(testutil.InvalidArgumentError{})
	// And failing CircuitBreaker
	cbBuilder := circuitbreaker.Builder[bool]().WithDelay(0)
	// And successful Fallback
	fbBuilder := fallback.BuilderOfResult[bool](true).HandleErrors(testutil.InvalidArgumentError{})
	stats := &listenerStats{}
	registerRpListeners(stats, rpBuilder)
	registerCbListeners(stats, cbBuilder)
	registerFbListeners(stats, fbBuilder)
	executor := failsafe.With[bool](fbBuilder.Build(), rpBuilder.Build(), cbBuilder.Build())
	registerExecutorListeners(stats, executor)

	// When
	executor.GetWithExecution(stub)

	// Then
	assert.Equal(t, 1, stats.rpSuccess)
	assert.Equal(t, 0, stats.rpFailure)

	assert.Equal(t, 0, stats.cbSuccess)
	assert.Equal(t, 1, stats.cbFailure)

	assert.Equal(t, 0, stats.fbFailedAttempt)
	assert.Equal(t, 1, stats.fbSuccess)
	assert.Equal(t, 0, stats.fbFailure)

	assert.Equal(t, 1, stats.complete)
	assert.Equal(t, 0, stats.success)
	assert.Equal(t, 1, stats.failure)
}

func TestListenersForFailingFallback(t *testing.T) {
	// Given - Fail 10 times
	stub := testutil.ErrorNTimesThenReturn[bool](testutil.InvalidStateError{}, 10)
	// Given successful RetryPolicy and CircuitBreaker
	rpBuilder := retrypolicy.Builder[bool]().HandleErrors(testutil.InvalidArgumentError{})
	cbBuilder := circuitbreaker.Builder[bool]().HandleErrors(testutil.InvalidArgumentError{}).WithDelay(0)
	// And failing Fallback
	fbBuilder := fallback.BuilderOfError[bool](testutil.ConnectionError{})
	stats := &listenerStats{}
	registerRpListeners(stats, rpBuilder)
	registerCbListeners(stats, cbBuilder)
	registerFbListeners(stats, fbBuilder)
	executor := failsafe.With[bool](fbBuilder.Build(), rpBuilder.Build(), cbBuilder.Build())
	registerExecutorListeners(stats, executor)

	// When
	executor.GetWithExecution(stub)

	// Then
	assert.Equal(t, 1, stats.rpSuccess)
	assert.Equal(t, 0, stats.rpFailure)

	assert.Equal(t, 1, stats.cbSuccess)
	assert.Equal(t, 0, stats.cbFailure)

	assert.Equal(t, 1, stats.fbFailedAttempt)
	assert.Equal(t, 0, stats.fbSuccess)
	assert.Equal(t, 1, stats.fbFailure)

	assert.Equal(t, 1, stats.complete)
	assert.Equal(t, 0, stats.success)
	assert.Equal(t, 1, stats.failure)
}

func TestGetElapsedTime(t *testing.T) {
	rp := retrypolicy.Builder[any]().
		HandleResult(false).
		OnRetryScheduled(func(e failsafe.ExecutionScheduledEvent[any]) {
			assert.True(t, e.GetElapsedAttemptTime().Milliseconds() >= 90)
		}).
		Build()
	failsafe.With[any](rp).Get(func() (any, error) {
		time.Sleep(100 * time.Millisecond)
		return false, nil
	})
}

func TestRetryPolicyOnScheduledRetry(t *testing.T) {
	executions := 0
	rp := retrypolicy.Builder[any]().HandleResult(nil).WithMaxRetries(1).OnFailedAttempt(func(e failsafe.ExecutionAttemptedEvent[any]) {
		if executions == 1 {
			assert.True(t, e.IsFirstAttempt())
			assert.False(t, e.IsRetry())
		} else {
			assert.False(t, e.IsFirstAttempt())
			assert.True(t, e.IsRetry())
		}
	}).OnRetry(func(e failsafe.ExecutionAttemptedEvent[any]) {
		assert.False(t, e.IsFirstAttempt())
		assert.True(t, e.IsRetry())
	}).OnRetryScheduled(func(e failsafe.ExecutionScheduledEvent[any]) {
		if executions == 1 {
			assert.True(t, e.IsFirstAttempt())
			assert.False(t, e.IsRetry())
		} else {
			assert.False(t, e.IsFirstAttempt())
			assert.True(t, e.IsRetry())
		}
	}).OnFailure(func(e failsafe.ExecutionCompletedEvent[any]) {
		assert.False(t, e.IsFirstAttempt())
		assert.True(t, e.IsRetry())
	}).Build()

	failsafe.With[any](rp).Get(func() (any, error) {
		executions++
		return nil, nil
	})
}

type listenerStats struct {
	// RetryPolicy
	abort           int
	rpFailedAttempt int
	retriesExceeded int
	retry           int
	retryScheduled  int
	rpSuccess       int
	rpFailure       int

	// CircuitBreaker
	open      int
	close     int
	halfOpen  int
	cbSuccess int
	cbFailure int

	// Fallback
	fbFailedAttempt int
	fbSuccess       int
	fbFailure       int

	// Executor
	complete int
	success  int
	failure  int
}

func registerRpListeners[R any](stats *listenerStats, rpBuilder retrypolicy.RetryPolicyBuilder[R]) {
	rpBuilder.OnAbort(func(f failsafe.ExecutionCompletedEvent[R]) {
		stats.abort++
	}).OnFailedAttempt(func(f failsafe.ExecutionAttemptedEvent[R]) {
		stats.rpFailedAttempt++
	}).OnRetriesExceeded(func(f failsafe.ExecutionCompletedEvent[R]) {
		stats.retriesExceeded++
	}).OnRetry(func(f failsafe.ExecutionAttemptedEvent[R]) {
		stats.retry++
	}).OnRetryScheduled(func(f failsafe.ExecutionScheduledEvent[R]) {
		stats.retryScheduled++
	}).OnSuccess(func(event failsafe.ExecutionCompletedEvent[R]) {
		stats.rpSuccess++
	}).OnFailure(func(event failsafe.ExecutionCompletedEvent[R]) {
		stats.rpFailure++
	})
}

func registerCbListeners[R any](stats *listenerStats, cbBuilder circuitbreaker.CircuitBreakerBuilder[R]) {
	cbBuilder.OnOpen(func(event circuitbreaker.StateChangedEvent) {
		stats.open++
	}).OnClose(func(event circuitbreaker.StateChangedEvent) {
		stats.close++
	}).OnHalfOpen(func(event circuitbreaker.StateChangedEvent) {
		stats.halfOpen++
	}).OnSuccess(func(event failsafe.ExecutionCompletedEvent[R]) {
		stats.cbSuccess++
	}).OnFailure(func(event failsafe.ExecutionCompletedEvent[R]) {
		stats.cbFailure++
	})
}

func registerFbListeners[R any](stats *listenerStats, fbBuilder fallback.FallbackBuilder[R]) {
	fbBuilder.OnFailedAttempt(func(f failsafe.ExecutionAttemptedEvent[R]) {
		stats.fbFailedAttempt++
	}).OnSuccess(func(event failsafe.ExecutionCompletedEvent[R]) {
		stats.fbSuccess++
	}).OnFailure(func(event failsafe.ExecutionCompletedEvent[R]) {
		stats.fbFailure++
	})
}

func registerExecutorListeners[R any](stats *listenerStats, executor failsafe.Executor[R]) {
	executor.OnComplete(func(e failsafe.ExecutionCompletedEvent[R]) {
		stats.complete++
	}).OnFailure(func(e failsafe.ExecutionCompletedEvent[R]) {
		stats.failure++
	}).OnSuccess(func(e failsafe.ExecutionCompletedEvent[R]) {
		stats.success++
	})
}