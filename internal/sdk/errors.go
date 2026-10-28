package sdk

import "fmt"

// ErrorCode 是统一错误码。
type ErrorCode string

const (
	ErrConfig       ErrorCode = "config"
	ErrManifest     ErrorCode = "manifest"
	ErrDependency   ErrorCode = "dependency"
	ErrPluginInit   ErrorCode = "plugin_init"
	ErrPluginStart  ErrorCode = "plugin_start"
	ErrPluginStop   ErrorCode = "plugin_stop"
	ErrPluginPanic  ErrorCode = "plugin_panic"
	ErrTimeout      ErrorCode = "timeout"
	ErrCanceled     ErrorCode = "canceled"
	ErrPermission   ErrorCode = "permission"
	ErrBudget       ErrorCode = "budget"
	ErrLLM          ErrorCode = "llm"
	ErrTool         ErrorCode = "tool"
	ErrAdapter      ErrorCode = "adapter"
	ErrRateLimited  ErrorCode = "rate_limited"
	ErrInvalidEvent ErrorCode = "invalid_event"
)

// Error 是内核/插件统一错误。
type Error struct {
	Code      ErrorCode
	Message   string
	UserText  string
	Retryable bool
	PluginID  string
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.PluginID != "" {
		return fmt.Sprintf("%s[%s]: %s", e.Code, e.PluginID, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }
