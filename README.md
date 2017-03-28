# Kubernetes Ingress Controller for AWS

This is an ingress controller for [Kubernetes](http://kubernetes.io/), the open-source container deployment, scaling, and management system. This controller, which is currently under active development, runs inside a Kubernetes cluster to monitor changes to your ingress resources and orchestrate [AWS Load Balancers](https://aws.amazon.com/elasticloadbalancing/) accordingly.

kube-ingress-aws-controller uses your EC2 instance metadata to find the CloudFormation stack that it belongs to, and then uses this information to discover the required AWS resources that are attached to each newly created [Application Load Balancer](https://aws.amazon.com/elasticloadbalancing/applicationloadbalancer/).

## Development Status

kube-ingress-aws-controller is a work in progress. As of March 2017 it's being used only in small production use cases at [Zalando](https://tech.zalando.com/), and is not yet battle-tested. However, we are actively seeking people to try it out and share feedback so we can make it better. We are also eager to bring new contributors on board; see our contributor guidelines here.

## Why We Created This

The project maintainers are building an infrastructure that runs [Kubernetes on top of AWS](https://github.com/zalando-incubator/kubernetes-on-aws) at large-scale (for nearly 200 delivery teams), and with automation. As such, we're creating our own tooling to support this new infrastructure. We couldn't find an existing ingress controller that operates like this one, so we created one ourselves.

We're using this ingress controller with [Skipper](http://github.com/zalando/skipper), an HTTP router that Zalando has used in production for more than a year as part of its front-end microservices architecture. Skipper's also open source, and you're free to use it, but you can use another ingress of your choosing along with this controller.

## Quickstart

### Deploying the Controller
Please refer to the [Deploy](#deploy) section for complete instructions.

## How This Controller Works

The controller continuously polls the API server checking for ingress resources. It runs an infinite loop and for each cycle it creates load balancers for new
ingress resources and deletes the load balancers for ingress resources that do not exist anymore.

The controller *will not* manage the security groups required to allow access from the internet to the load balancers as it assumes that their lifecycle is external to the controller itself.

### Discovery

On startup the controller discovers the AWS resources relevant for the controller operations:

1. The AutoScalingGroup

    Simple lookup of the Autoscaling Group which name matches the "aws:autoscaling:groupName" tag from the EC2 instance running the
    controller.

2. The Security Group

    Lookup of the "Name" tag of the Security Group matching the stack for the controller node and the tag "aws:cloudformation:logical-id"
    matching the value "IngressLoadBalancerSecurityGroup"

### Creating Load Balancers

When the controller learns about new ingress resources, it uses the host specified in it to automatically determine the most specific certificate to use.
An example ingress is the following:

```
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: test-app
spec:
  rules:
  - host: test-app.example.org
    http:
      paths:
      - backend:
          serviceName: test-app-service
          servicePort: main-port
```

The Application Load Balancer that will be created by the controller, will have both an HTTP listener and an HTTPS listener. The latter, will use the automatically selected certificate.

Alternatively, you can specify the Amazon Resource Name (ARN) of the desired certificate can be specified with an annotation like in the following example:

```
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: myingress
  annotations:
    zalando.org/aws-load-balancer-ssl-cert: arn:aws:acm:eu-central-1:123456789012:certificate/f4bd7ed6-bf23-11e6-8db1-ef7ba1500c61
spec:
  rules:
  - host: test-app.example.org
    http:
      paths:
      - backend:
          serviceName: test-app-service
          servicePort: main-port
```

Application Load Balancers created by the controller have a custom Tag marking them as managed load balancers to differentiate them from other load balancers. The tag looks like the following:

    kubernetes:application: kube-ingress-aws-controller

They also share the "ClusterID" tag as the other resources from the same CloudFormation stack. The load balancer names
are derived from a truncated SHA1 hash of the certificate ARN, combined with a normalized version of the ClusterID.
Due to the 32 characters limitation, it can be truncated.

### Deleting load balancers

When the controller detects that a managed load balancer for the current cluster doesn't have a matching ingress
resource anymore, it deletes all the previously created resources.

## Building

This project provides a `Makefile` that can be used to build either a binary or to build a docker image for it.

### Building a Binary

To build a binary for your current operating system, it is enough to run `make` or `make build.linux` to build a binary for Linux.

### Building a Docker Image

To create a docker image, you can execute `make build.docker` instead. You can then push your docker image to the docker registry of your choice.

## Deploy

To deploy the ingress controller, you can use [this yaml](deploy/ingress-controller.yaml) as descriptor.

The image used in the yaml can be customized, we provide `registry.opensource.zalan.do/teapot/kube-aws-ingress-controller:latest` as a public usable docker image built from this codebase.

You can deploy it by executing the following command, after replacing the placeholder for the region:

```
kubectl apply -f deploy/ingress-controller.yaml
```

## Trying it out

The Ingress Controller responsibility is limited to managing load balancers as described above. To have a fully functional setup, additionally to the ingress controller, you can use [skipper](https://github.com/zalando/skipper) to route the traffic to the application. The setup follows what described [here](https://kubernetes-on-aws.readthedocs.io/en/latest/user-guide/ingress.html).

You can deploy `skipper` as a `DaemonSet` using [this yaml](deploy/skipper.yaml).

To deploy it, you can execute the following commands:

```
kubectl apply -f deploy/skipper.yaml
```

To complete the setup, some additional regarding security groups and IAM roles are needed and are described in the following document: [requirements](deploy/requirements.md).

### DNS

While not strictly necessary for the `Kubernetes Ingress Controller` to work, to have convenient DNS names for your application, you can use [mate](https://github.com/zalando-incubator/mate).

NOTE: `mate` will soon be replaced by a new Kubernetes incubator project, [external-dns](https://github.com/kubernetes-incubator/external-dns).

## Contributing

We welcome your contributions, ideas and bug reports via issues and pull requests. Please open an issue first to discuss possible problems and features.

## Contact

You can contact the maintainers of the project via email at the address contained in the [MAINTAINERS file](MAINTAINERS).

## License

The MIT License (MIT) Copyright © [2016] Zalando SE, https://tech.zalando.com
Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
