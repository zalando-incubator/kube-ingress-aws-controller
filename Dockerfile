FROM registry.opensource.zalan.do/stups/alpine:UPSTREAM
MAINTAINER Team Teapot @ Zalando SE <team-teapot@zalando.de>

# add scm-source
ADD scm-source.json /

# add binary
ADD build/linux/kube_ingress_aws_controller /

ENTRYPOINT ["/kube_ingress_aws_controller"]
