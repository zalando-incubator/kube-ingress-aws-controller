FROM registry.opensource.zalan.do/library/alpine-3.12:latest
LABEL maintainer="Team Teapot @ Zalando SE <team-teapot@zalando.de>"

ADD build/linux/kube-ingress-aws-controller /

ENTRYPOINT ["/kube-ingress-aws-controller"]
