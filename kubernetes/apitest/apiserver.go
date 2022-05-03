package apitest

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ApiHandler func(t *testing.T, w http.ResponseWriter, r *http.Request)

type ApiServer struct {
	*httptest.Server
	t          *testing.T
	pathPrefix string
	handlers   map[string]ApiHandler
	served     map[string]int
}

func NewApiServer(t *testing.T, pathPrefix string, handlers map[string]ApiHandler) *ApiServer {
	s := &ApiServer{
		t:          t,
		pathPrefix: pathPrefix,
		handlers:   handlers,
		served:     make(map[string]int),
	}
	s.Server = httptest.NewServer(s)
	return s
}

func (s *ApiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	op := fmt.Sprintf("%s %s", r.Method, strings.TrimPrefix(r.URL.Path, s.pathPrefix))

	if handler, ok := s.handlers[op]; ok {
		handler(s.t, w, r)
		s.served[op]++
	} else {
		http.Error(w, "unsupported operation", http.StatusInternalServerError)
	}
}

func (s *ApiServer) Close() {
	s.Server.Close()

	s.t.Helper()

	expected := make(map[string]int)
	for op := range s.handlers {
		expected[op] = 1
	}
	assert.Equal(s.t, expected, s.served)
}

func JsonFromYamlHandler(filename string) ApiHandler {
	return func(_ *testing.T, w http.ResponseWriter, _ *http.Request) {
		bytes, err := yamlToJson(filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(bytes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func ExpectJsonBodyAsYaml(t *testing.T, r *http.Request, filename string) {
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)

	expected, err := yamlToJson(filename)
	require.NoError(t, err)

	assert.JSONEq(t, string(expected), string(body))
}

func yamlToJson(filename string) ([]byte, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return yaml.YAMLToJSON(bytes)
}
