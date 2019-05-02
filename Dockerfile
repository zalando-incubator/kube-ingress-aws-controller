FROM registry.opensource.zalan.do/stups/alpine:latest
MAINTAINER Team Teapot @ Zalando SE <team-teapot@zalando.de>

ADD build/linux/kube-ingress-aws-controller /

ENTRYPOINT ["/kube-ingress-aws-controller"]
