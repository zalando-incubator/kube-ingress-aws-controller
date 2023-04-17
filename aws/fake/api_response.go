package fake

import "errors"

type APIResponse struct {
	response interface{}
	err      error
}

type Tags map[string]string

var ErrDummy = errors.New("fail")

func R(r interface{}, e error) *APIResponse {
	return &APIResponse{response: r, err: e}
}
