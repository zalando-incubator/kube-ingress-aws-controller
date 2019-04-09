# builder image
FROM golang as builder

WORKDIR /github.com/zalando-incubator/kube-ingress-aws-controller
COPY . .
RUN make build.linux

# final image
FROM registry.opensource.zalan.do/stups/alpine:latest
MAINTAINER Team Teapot @ Zalando SE <team-teapot@zalando.de>

COPY --from=builder /github.com/zalando-incubator/kube-ingress-aws-controller/build/linux/kube-ingress-aws-controller \
  /bin/kube-ingress-aws-controller

ENTRYPOINT ["/bin/kube-ingress-aws-controller"]
