# Ingress annotations

Overview of annotations which can be set.

## Annotations
|Name                       | Type |Default
|---------------------------|------|------|
|[alb.ingress.kubernetes.io/ip-address-type](#ip-address-type)|ipv4 \| dualstack|ipv4|
|zalando.org/aws-load-balancer-ssl-cert|string|N/A|
|zalando.org/aws-load-balancer-scheme|internal \| internet-facing |internet-facing|
|zalando.org/aws-load-balancer-shared|true \| false|false|
|zalando.org/aws-load-balancer-security-group|string|N/A|
|zalando.org/aws-load-balancer-ssl-policy|string|ELBSecurityPolicy-2016-08|
|kubernetes.io/ingress.class|string|N/A|