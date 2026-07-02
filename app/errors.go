package app

import "fmt"

type ErrCode string

const (
	ErrCodeEnv        ErrCode = "env"
	ErrCodeParam      ErrCode = "param"
	ErrCodeTranscribe ErrCode = "transcribe"
	ErrCodeLLM        ErrCode = "llm"
	ErrCodeExport     ErrCode = "export"
	ErrCodeCanceled   ErrCode = "canceled"
	ErrCodeInternal   ErrCode = "internal"
)

type AppError struct {
	Code    ErrCode `json:"code"`
	Message string  `json:"message"`
	Detail  string  `json:"detail"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewAppError(code ErrCode, message string, detail string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Detail:  detail,
	}
}

func WrapError(code ErrCode, err error) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{
		Code:    code,
		Message: err.Error(),
		Detail:  err.Error(),
	}
}