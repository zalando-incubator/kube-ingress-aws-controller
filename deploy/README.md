# Deploy

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

## Configuration
* Update the file ingress-controller.yaml.example following your needs and rename it to ingress-controller.yaml
* Update the file skipper.yaml.example following your needs and rename it to skipper.yaml

## Install

    # create a specific service account for skipper and ingress
    % kubectl apply -f skipper_serviceaccount.yaml
    % kubectl apply -f ingress-controller.yaml
    # install skipper an ingress http router
    % kubectl create -f deploy/skipper.yaml
    # install the controller which glues together AWS and the ingress implementation
    % kubectl create -f deploy/ingress-controller.yaml

For more information regarding skipper's requirements have a look here [ingress-controller](https://opensource.zalando.com/skipper/kubernetes/ingress-controller/).

If you have done this, you can use our
[example](https://github.com/zalando-incubator/kube-ingress-aws-controller/tree/master/example)
to test the integration and see our [test deployment for advanced features](test-deployment.md).

## Test deployment

To test base and advanced features, please follow [this guide](test-deployment.md).
