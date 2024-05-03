ARG BASE_IMAGE=registry.opensource.zalan.do/library/amazonlinux-2023:latest
FROM ${BASE_IMAGE}
LABEL maintainer="Team Teapot @ Zalando SE <team-teapot@zalando.de>"

ARG TARGETARCH

ADD build/linux/${TARGETARCH}/kube-ingress-aws-controller /

ENTRYPOINT ["/kube-ingress-aws-controller"]
