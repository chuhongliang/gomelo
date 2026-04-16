package lib

import "fmt"

type ErrCode int

const (
	ErrCodeOK           ErrCode = 0
	ErrCodeInvalidParam ErrCode = 400
	ErrCodeUnauthorized ErrCode = 401
	ErrCodeForbidden    ErrCode = 403
	ErrCodeNotFound     ErrCode = 404
	ErrCodeTimeout      ErrCode = 408
	ErrCodeConflict     ErrCode = 409
	ErrCodeServerErr    ErrCode = 500
	ErrCodeUnavailable  ErrCode = 503
)

func (e ErrCode) String() string {
	switch e {
	case ErrCodeOK:
		return "ok"
	case ErrCodeInvalidParam:
		return "invalid parameter"
	case ErrCodeUnauthorized:
		return "unauthorized"
	case ErrCodeForbidden:
		return "forbidden"
	case ErrCodeNotFound:
		return "not found"
	case ErrCodeTimeout:
		return "timeout"
	case ErrCodeConflict:
		return "conflict"
	case ErrCodeServerErr:
		return "internal server error"
	case ErrCodeUnavailable:
		return "service unavailable"
	default:
		return "unknown error"
	}
}

type RPCError struct {
	Code    ErrCode `json:"code"`
	Message string  `json:"msg"`
}

func NewRPCError(code ErrCode, msg string) *RPCError {
	return &RPCError{Code: code, Message: msg}
}

func NewRPCErrorf(code ErrCode, format string, args ...any) *RPCError {
	return &RPCError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("[%d] %s: %s", e.Code, e.Code.String(), e.Message)
}

func (e *RPCError) ToMap() map[string]any {
	return map[string]any{
		"code": int(e.Code),
		"msg":  e.Message,
	}
}

type HandlerResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func NewHandlerResponse(code ErrCode, data any) *HandlerResponse {
	return &HandlerResponse{
		Code: int(code),
		Msg:  code.String(),
		Data: data,
	}
}

func NewHandlerResponseWithMsg(code ErrCode, msg string, data any) *HandlerResponse {
	return &HandlerResponse{
		Code: int(code),
		Msg:  msg,
		Data: data,
	}
}

func OK(data any) *HandlerResponse { return NewHandlerResponse(ErrCodeOK, data) }
func InvalidParam(msg string) *HandlerResponse {
	return NewHandlerResponseWithMsg(ErrCodeInvalidParam, msg, nil)
}
func Unauthorized(msg string) *HandlerResponse {
	return NewHandlerResponseWithMsg(ErrCodeUnauthorized, msg, nil)
}
func Forbidden(msg string) *HandlerResponse {
	return NewHandlerResponseWithMsg(ErrCodeForbidden, msg, nil)
}
func NotFound(msg string) *HandlerResponse {
	return NewHandlerResponseWithMsg(ErrCodeNotFound, msg, nil)
}
func ServerError(msg string) *HandlerResponse {
	return NewHandlerResponseWithMsg(ErrCodeServerErr, msg, nil)
}
