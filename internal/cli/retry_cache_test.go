package cli

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryOperation_Success(t *testing.T) {
	err := retryOperation(func() error { return nil }, 3, time.Millisecond)
	assert.NoError(t, err)
}

func TestRetryOperation_SuccessOnRetry(t *testing.T) {
	attempts := 0
	err := retryOperation(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("fail")
		}
		return nil
	}, 3, time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetryOperation_AllFail(t *testing.T) {
	lastErr := errors.New("third")
	count := 0
	err := retryOperation(func() error {
		count++
		if count == 3 {
			return lastErr
		}
		return errors.New("other")
	}, 3, time.Millisecond)
	require.Error(t, err)
	assert.Equal(t, "third", err.Error())
}

func TestRetryOperation_SingleAttempt(t *testing.T) {
	expected := errors.New("only")
	err := retryOperation(func() error { return expected }, 1, time.Millisecond)
	require.Error(t, err)
	assert.Equal(t, "only", err.Error())
}

func TestCacheClear(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	err := cacheClearCmd.RunE(cacheClearCmd, []string{})
	w.Close()
	os.Stdout = old
	require.NoError(t, err)
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	assert.Contains(t, string(buf[:n]), "all caches cleared")
}
