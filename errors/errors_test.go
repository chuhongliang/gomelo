package errors

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(RouteNotFound, "route not found")
	if err.Code != RouteNotFound {
		t.Errorf("expected code=%d, got %d", RouteNotFound, err.Code)
	}
	if err.Message != "route not found" {
		t.Errorf("expected message='route not found', got '%s'", err.Message)
	}
}

func TestNewf(t *testing.T) {
	err := Newf(BadRequest, "invalid param: %s", "name")
	if err.Code != BadRequest {
		t.Errorf("expected code=%d, got %d", BadRequest, err.Code)
	}
}

func TestWrap(t *testing.T) {
	inner := errors.New("inner error")
	err := Wrap(RPCError, inner, "rpc failed")
	if err.Code != RPCError {
		t.Errorf("expected code=%d, got %d", RPCError, err.Code)
	}
	if err.Err != inner {
		t.Error("expected Err to be inner error")
	}
}

func TestWrapf(t *testing.T) {
	inner := errors.New("inner error")
	err := Wrapf(RPCError, inner, "rpc call %s failed", "AddUser")
	if err.Code != RPCError {
		t.Errorf("expected code=%d, got %d", RPCError, err.Code)
	}
}

func TestGomeloError_Error(t *testing.T) {
	err := New(RouteNotFound, "route not found")
	expected := "[1001] route not found"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

func TestGomeloError_ErrorWithErr(t *testing.T) {
	inner := errors.New("inner error")
	err := Wrap(ServerError, inner, "server error")
	result := err.Error()
	if result != "[500] server error: inner error" {
		t.Errorf("unexpected error format: %s", result)
	}
}

func TestGomeloError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := Wrap(RPCError, inner, "rpc failed")
	if err.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}
}

func TestGomeloError_WithDetail(t *testing.T) {
	err := New(BadRequest, "bad request").WithDetail("field 'name' is required")
	if err.Detail != "field 'name' is required" {
		t.Errorf("expected detail='field 'name' is required', got '%s'", err.Detail)
	}
}

func TestGomeloError_WithErr(t *testing.T) {
	inner := errors.New("inner error")
	err := New(BadRequest, "bad request").WithErr(inner)
	if err.Err != inner {
		t.Error("expected Err to be inner error")
	}
}

func TestGomeloError_ToMap(t *testing.T) {
	err := New(BadRequest, "bad request")
	m := err.ToMap()
	if m["code"] != BadRequest {
		t.Errorf("expected code=%d, got %v", BadRequest, m["code"])
	}
	if m["msg"] != "bad request" {
		t.Errorf("expected msg='bad request', got %v", m["msg"])
	}
}

func TestGomeloError_ToMap_WithDetail(t *testing.T) {
	err := New(BadRequest, "bad request").WithDetail("extra info")
	m := err.ToMap()
	if m["detail"] != "extra info" {
		t.Errorf("expected detail='extra info', got %v", m["detail"])
	}
}

func TestCode_Error(t *testing.T) {
	code := RouteNotFound
	if code.Error() != "Route not found" {
		t.Errorf("expected 'Route not found', got '%s'", code.Error())
	}
}

func TestCode_WithMessage(t *testing.T) {
	err := BadRequest.WithMessage("invalid input")
	if err.Code != BadRequest {
		t.Errorf("expected code=%d, got %d", BadRequest, err.Code)
	}
	if err.Message != "invalid input" {
		t.Errorf("expected message='invalid input', got '%s'", err.Message)
	}
}

func TestCode_WithMessagef(t *testing.T) {
	err := BadRequest.WithMessagef("invalid %s", "param")
	if err.Message != "invalid param" {
		t.Errorf("expected message='invalid param', got '%s'", err.Message)
	}
}

func TestCode_WithError(t *testing.T) {
	inner := errors.New("inner")
	err := RPCError.WithError(inner)
	if err.Code != RPCError {
		t.Errorf("expected code=%d, got %d", RPCError, err.Code)
	}
	if err.Err != inner {
		t.Error("expected Err to be inner error")
	}
	if err.Message != "RPC error" {
		t.Errorf("expected message='RPC error', got '%s'", err.Message)
	}
}

func TestCode_WithDetail(t *testing.T) {
	err := BadRequest.WithDetail("extra info")
	if err.Detail != "extra info" {
		t.Errorf("expected detail='extra info', got '%s'", err.Detail)
	}
}

func TestIsCode(t *testing.T) {
	err := New(RouteNotFound, "route not found")
	if !IsCode(err, RouteNotFound) {
		t.Error("expected IsCode to return true for RouteNotFound")
	}
	if IsCode(err, HandlerNotFound) {
		t.Error("expected IsCode to return false for HandlerNotFound")
	}
}

func TestIsCode_WithWrappedError(t *testing.T) {
	inner := New(RouteNotFound, "route not found")
	err := Wrap(RPCError, inner, "rpc failed")
	if IsCode(err, RouteNotFound) {
		t.Error("expected IsCode to return false for wrapped error")
	}
	if !IsCode(err, RPCError) {
		t.Error("expected IsCode to find RPCError")
	}
}

func TestIsCode_NilError(t *testing.T) {
	if IsCode(nil, RouteNotFound) {
		t.Error("expected IsCode to return false for nil error")
	}
}

func TestGetMessage(t *testing.T) {
	tests := []struct {
		code    Code
		message string
	}{
		{OK, "OK"},
		{BadRequest, "Bad Request"},
		{Unauthorized, "Unauthorized"},
		{Forbidden, "Forbidden"},
		{NotFound, "Not Found"},
		{ServerError, "Internal Server Error"},
		{RouteNotFound, "Route not found"},
		{HandlerNotFound, "Handler not found"},
		{SessionExpired, "Session expired"},
		{RPCTimeout, "RPC timeout"},
		{AuthError, "Auth error"},
		{GameError, "Game error"},
		{PlayerNotFound, "Player not found"},
	}

	for _, tt := range tests {
		if msg := GetMessage(tt.code); msg != tt.message {
			t.Errorf("GetMessage(%d) = '%s', want '%s'", tt.code, msg, tt.message)
		}
	}
}

func TestGetMessage_Unknown(t *testing.T) {
	code := Code(9999)
	if msg := GetMessage(code); msg != "Unknown error" {
		t.Errorf("expected 'Unknown error', got '%s'", msg)
	}
}

func TestToHTTPStatus(t *testing.T) {
	tests := []struct {
		code   Code
		status int
	}{
		{OK, 200},
		{BadRequest, 400},
		{Unauthorized, 401},
		{Forbidden, 403},
		{NotFound, 404},
		{ServerError, 500},
		{GameError, 500},
	}

	for _, tt := range tests {
		if status := ToHTTPStatus(tt.code); status != tt.status {
			t.Errorf("ToHTTPStatus(%d) = %d, want %d", tt.code, status, tt.status)
		}
	}
}

func TestToHTTPStatus_RouteNotFound(t *testing.T) {
	status := ToHTTPStatus(RouteNotFound)
	if status != 500 {
		t.Errorf("expected 500 for RouteNotFound (1001), got %d", status)
	}
}

func TestNewResponse(t *testing.T) {
	resp := NewResponse(nil)
	if resp.Code != OK {
		t.Errorf("expected code=OK, got %d", resp.Code)
	}
	if resp.Msg != "OK" {
		t.Errorf("expected msg='OK', got '%s'", resp.Msg)
	}
}

func TestNewResponse_WithError(t *testing.T) {
	err := New(BadRequest, "bad request")
	resp := NewResponse(err)
	if resp.Code != BadRequest {
		t.Errorf("expected code=%d, got %d", BadRequest, resp.Code)
	}
	if resp.Msg != "bad request" {
		t.Errorf("expected msg='bad request', got '%s'", resp.Msg)
	}
}

func TestNewResponse_WithUnknownError(t *testing.T) {
	err := errors.New("unknown error")
	resp := NewResponse(err)
	if resp.Code != ServerError {
		t.Errorf("expected code=ServerError, got %d", resp.Code)
	}
}

func TestNewResponseWithData(t *testing.T) {
	resp := NewResponseWithData(map[string]string{"key": "value"})
	if resp.Code != OK {
		t.Errorf("expected code=OK, got %d", resp.Code)
	}
	if resp.Data == nil {
		t.Error("expected Data to be set")
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(NotFound, "resource not found")
	if resp.Code != NotFound {
		t.Errorf("expected code=%d, got %d", NotFound, resp.Code)
	}
	if resp.Msg != "resource not found" {
		t.Errorf("expected msg='resource not found', got '%s'", resp.Msg)
	}
}
