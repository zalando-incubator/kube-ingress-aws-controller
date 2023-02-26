package fake

import "errors"

type ApiResponse struct {
	response interface{}
	err      error
}

type Tags map[string]string

var ErrDummy = errors.New("fail")

func R(r interface{}, e error) *ApiResponse {
	return &ApiResponse{response: r, err: e}
}
