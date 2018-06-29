# builder image
FROM golang as builder

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
WORKDIR /go/src/github.com/zalando-incubator/kube-ingress-aws-controller
COPY . .
RUN dep ensure -v -vendor-only
RUN make test
RUN make build.linux

# final image
FROM registry.opensource.zalan.do/stups/alpine:latest
MAINTAINER Team Teapot @ Zalando SE <team-teapot@zalando.de>

COPY --from=builder /go/src/github.com/zalando-incubator/kube-ingress-aws-controller/build/linux/kube-ingress-aws-controller \
  /bin/kube-ingress-aws-controller

ENTRYPOINT ["/bin/kube-ingress-aws-controller"]
