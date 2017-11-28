# Deploy

If you use [Kops](https://github.com/kubernetes/kops) to create your
cluster, please use our [deployment guide for Kops](kops.md)

## Requirements

* You have a running Kubernetes Cluster on AWS.
* You have a route53 Hosted Zone in your AWS account.
* You have provisioned ACM or IAM certificates that are valid
  for example one wildcard certificate for `*.YOUR_HOSTED_ZONE`.
* You have met all [requirements.md](requirements.md), such that the
  ingress controller has access to all relevant AWS APIs.
* **Optional** to manage route53 DNS records automatically you can install
  [external-dns](https://github.com/kubernetes-incubator/external-dns/)
  to manage DNS records for your Ingress specifications.

## Install

    % cd deploy
    # install the ingress implementation
    % kubectl create -f skipper.yaml
    # install the controller which glues together AWS and the ingress implementation
    % kubectl create -f ingress-controller.yaml

If you have done this, you can use our
[example](https://github.com/zalando-incubator/kube-ingress-aws-controller/tree/master/example)
to test the integration.
