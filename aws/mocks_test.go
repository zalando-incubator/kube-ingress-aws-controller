package aws

import "errors"

type apiResponse struct {
	response interface{}
	err      error
}

type tags map[string]string

type awsTags map[string]tags

var errDummy = errors.New("fail")

func R(r interface{}, e error) *apiResponse {
	return &apiResponse{response: r, err: e}
}
