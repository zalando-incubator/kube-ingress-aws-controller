package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestListNodes(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		f, _ := os.Open("testdata/fixture02.json")
		defer f.Close()
		rw.WriteHeader(http.StatusOK)
		io.Copy(rw, f)
	}))
	defer testServer.Close()
	kubeClient, _ := newSimpleClient(&Config{BaseURL: testServer.URL})
	want := newNodeList(newNode("ip-1-2-3-4.ec2.internal", "1.2.3.4"))
	got, err := listNode(kubeClient)
	if err != nil {
		t.Errorf("unexpected error from listNode: %v", err)
	} else {
		if !reflect.DeepEqual(got, want) {
			t.Logf("%v", *want.Items[0])
			t.Logf("%v", *got.Items[0])
			t.Errorf("unexpected result from listNode. wanted %v, got %v", want, got)
		}
	}
}

func TestListNodeFailureScenarios(t *testing.T) {
	for _, test := range []struct {
		statusCode int
		body       string
	}{
		{http.StatusInternalServerError, "{}"},
		{http.StatusOK, "`"},
	} {
		t.Run(fmt.Sprintf("%v", test.statusCode), func(t *testing.T) {
			testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(test.statusCode)
				fmt.Fprintln(rw, test.body)
			}))
			defer testServer.Close()
			cfg := &Config{BaseURL: testServer.URL}
			kubeClient, _ := newSimpleClient(cfg)

			_, err := listNode(kubeClient)
			if err == nil {
				t.Error("expected an error but list node call succeeded")
			}
		})
	}
}

func TestNodeGetAddress(t *testing.T) {
	node := &node{Status: nodeStatus{Addresses: []nodeAddress{nodeAddress{Address: "1.2.3.4", Type: nodeAddressTypeInternalIP}, nodeAddress{Address: "ip-1-2-3-4.ec2.internal", Type: nodeAddressTypeHostname}}}}
	for _, test := range []struct {
		key  string
		want string
	}{
		{nodeAddressTypeInternalIP, "1.2.3.4"},
		{nodeAddressTypeHostname, "ip-1-2-3-4.ec2.internal"},
		{"unknown", ""},
	} {
		t.Run(fmt.Sprintf("%s/%s", test.key, test.want), func(t *testing.T) {
			if got := node.getAddress(test.key); got != test.want {
				t.Errorf("unexpected address type '%s' value. wanted %q, got %q", test.key, test.want, got)
			}

		})
	}
}

func newNodeList(nodes ...*node) *nodeList {
	ret := nodeList{
		APIVersion: "v1",
		Kind:       "NodeList",
		Items:      nodes,
	}
	return &ret
}

func newNode(name string, internalIp string) *node {
	ret := node{
		Metadata: nodeMetadata{
			Name:            name,
			Uid:             "fixture02",
			Annotations:     map[string]interface{}{},
			SelfLink:        "/api/v1/nodes/" + name,
			ResourceVersion: "42",
			Labels:          map[string]interface{}{"kubernetes.io/role": "node"},
		},
		Status: nodeStatus{
			Addresses: []nodeAddress{
				nodeAddress{
					Address: internalIp,
					Type:    nodeAddressTypeInternalIP,
				},
				nodeAddress{
					Address: name,
					Type:    nodeAddressTypeHostname,
				},
			},
		},
		Spec: nodeSpec{
			ExternalId: "i-0123456789abcdef0",
			PodCIDR:    "100.100.100.0/24",
			ProviderId: "aws:///us-east-1c/i-0123456789abcdef0",
		},
	}
	return &ret
}
