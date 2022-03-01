module github.com/zalando-incubator/kube-ingress-aws-controller

go 1.16

require (
	github.com/alecthomas/units v0.0.0-20210208195552-ff826a37aa15 // indirect
	github.com/aws/aws-sdk-go v1.42.44
	github.com/ghodss/yaml v1.0.0
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/google/go-cmp v0.5.7
	github.com/google/uuid v1.3.0
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/linki/instrumented_http v0.3.0
	github.com/mweagle/go-cloudformation v0.0.0-20210117063902-00aa242fdc67
	github.com/prometheus/client_golang v1.12.1
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/zalando/skipper v0.13.178
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
	gopkg.in/go-playground/validator.v9 v9.31.0 // indirect
	k8s.io/api v0.23.4
	k8s.io/apimachinery v0.23.4
	k8s.io/client-go v0.22.0
)
