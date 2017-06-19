# builder image
FROM golang:1.8 as builder

RUN go get github.com/Masterminds/glide
WORKDIR /go/src/github.com/zalando-incubator/kube-ingress-aws-controller
COPY . .
RUN glide install --strip-vendor
RUN make build.linux

# final image
FROM registry.opensource.zalan.do/stups/alpine:latest
MAINTAINER Team Teapot @ Zalando SE <team-teapot@zalando.de>

COPY --from=builder /go/src/github.com/zalando-incubator/kube-ingress-aws-controller/build/linux/kube-ingress-aws-controller \
  /bin/kube-ingress-aws-controller

ENTRYPOINT ["/bin/kube-ingress-aws-controller"]
