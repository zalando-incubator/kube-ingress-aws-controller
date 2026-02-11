# Kubernetes Ingress Controller for AWS

This is an ingress controller for [Kubernetes](http://kubernetes.io/) — the open-source container deployment,
scaling, and management system — on AWS. It runs inside a Kubernetes cluster to monitor changes to your ingress
resources and orchestrate [AWS Load Balancers](https://aws.amazon.com/elasticloadbalancing/) accordingly.

[![Build Status](https://github.com/zalando-incubator/kube-ingress-aws-controller/workflows/ci/badge.svg)](https://github.com/zalando-incubator/kube-ingress-aws-controller/actions?query=branch:master)
[![Coverage Status](https://coveralls.io/repos/github/zalando-incubator/kube-ingress-aws-controller/badge.svg?branch=master)](https://coveralls.io/github/zalando-incubator/kube-ingress-aws-controller?branch=master)
[![GitHub release](https://img.shields.io/github/release/zalando-incubator/kube-ingress-aws-controller.svg)](https://github.com/zalando-incubator/kube-ingress-aws-controller/releases)
[![go-doc](https://pkg.go.dev/github.com/zalando-incubator/kube-ingress-aws-controller?status.svg)](https://pkg.go.dev/github.com/zalando-incubator/kube-ingress-aws-controller)


This ingress controller uses the EC2 instance metadata of the worker node where it's currently running to find the
additional details about the cluster provisioned by [Kubernetes on top of AWS](https://github.com/zalando-incubator/kubernetes-on-aws).
This information is used to manage AWS resources for each ingress objects of the cluster.

## Features

- Uses CloudFormation to guarantee consistent state
- Automatic discovery of SSL certificates
- Automatic forwarding of requests to all Worker Nodes, even with auto scaling
- Automatic cleanup of unnecessary managed resources
- Support for both [Application Load Balancers][alb] and [Network Load Balancers][nlb].
- Support for internet-facing and internal load balancers
- Support for ignoring cluster-internal ingress, that only have `--cluster-local-domain=cluster.local` domains.
- Support for denying traffic for internal domains.
- Support for multiple Auto Scaling Groups
- Support for instances that are not part of Auto Scaling Group
- Support for SSLPolicy, set default and per ingress
- Support for [CloudWatch Alarm configuration](cloudwatch.md)
- Can be used in clusters created by [Kops](https://github.com/kubernetes/kops), see our [deployment guide for Kops](deploy/kops.md)
- [Support Multiple TLS Certificates per ALB (SNI)](https://aws.amazon.com/blogs/aws/new-application-load-balancer-sni/).
- Support for AWS WAF and WAFv2
- Support for AWS CNI pod direct access
- Support for Kubernetes CRD [RouteGroup](https://opensource.zalando.com/skipper/kubernetes/routegroups/)
- Support for zone aware traffic (defaults to cross zone traffic and no zone affinity)
   - enable and disable cross zone traffic: `--nlb-cross-zone=false`
   - set zone affinity to resolve DNS to same zone: `--nlb-zone-affinity=availability_zone_affinity`, see also [NLB attributes](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html\#load-balancer-attributes) and [NLB zonal DNS affinity](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html\#zonal-dns-affinity)
- Support for explicitly enable certificates by using certificate Tags `--cert-filter-tag=key=value`
- Suppport for `ipv4` and `dualstack` ip address types for ALB and NLB
    - set default ip address type for both ALB and NLB using `--ip-addr-type=dualstack`
    - set specific ip address type for a particular ALB or NLB by using the annotation `alb.ingress.kubernetes.io/ip-address-type: dualstack` in the ingress of the resource

## Upgrade

### <v0.18.14 to >=v0.18.14

Version `v0.18.14` adds support for IPv6 target group IP address type. When using IPv6 targets, ensure your load balancer is configured as dualstack (`--ip-addr-type=dualstack` or `alb.ingress.kubernetes.io/ip-address-type: dualstack`). IPv4-only load balancers cannot route to IPv6 targets and will fail with a clear error message.

### <v0.18.0 to >=v0.18.0

Version `v0.18.0` vendors-in https://github.com/mweagle/go-cloudformation library to enable addition of missing CloudFormation features.
It was last updated in October 2021 and recommends to use https://github.com/awslabs/goformation which is archived since October 2024.

### <v0.17.0 to >=v0.17.0

Version `v0.17.0` uses the controller context in the AWS Adapter API's, so as to pass context to the AWS SDK Go v2.

### <v0.16.0 to >=v0.16.0

Version `v0.16.0` migrates to use the AWS SDK Go v2 as the version v1's End of life
approaches (July 31, 2025)

### <v0.15.0 to >=v0.15.0

Version `v0.15.0` removes support for deprecated Ingress versions
`extensions/v1beta1` and `networking.k8s.io/v1beta1`.

### <v0.14.0 to >=v0.14.0

Version `v0.14.0` makes `target-access-mode` flag required to make upgrading users aware of the [issue](https://github.com/zalando-incubator/kube-ingress-aws-controller/issues/507).

New deployment of the controller should use `--target-access-mode=HostPort` or `--target-access-mode=AWSCNI`.

To upgrade from `<v0.12.17` use `--target-access-mode=Legacy` - it is the same as `HostPort` but does not set target type and
relies on CloudFormation to use `instance` as a default value.

Note that changing later from `--target-access-mode=Legacy` will change target type in CloudFormation and trigger target group recreation and downtime.

To upgrade from `>=v0.12.17` when `--target-access-mode` is not set use explicit `--target-access-mode=HostPort`.

### <v0.13.0 to >=0.13.0

Version `v0.13.0` use Ingress version v1 as default. You can downgrade
ingress version to earlier versions via flag. You will also need to
allow the access via RBAC, see more information in [<v0.11.0 to >=0.11.0](#v0110-to-0110) below.

### <v0.12.17 to <v0.14.0

Please see [release note](https://github.com/zalando-incubator/kube-ingress-aws-controller/releases/tag/v0.12.17)
and [issue](https://github.com/zalando-incubator/kube-ingress-aws-controller/issues/507)
this update can cause 30s downtime, if you don't use AWS CNI mode.

Please upgrade to `>=v0.14.0`.

### <v0.12.0 to <=0.12.16

Version `v0.12.0` changes Network Load Balancer type handling if Application Load Balancer type feature is requested. See [Load Balancers types](#load-balancers-types) notes for details.

### <v0.11.0 to >=0.11.0

Version `v0.11.0` changes the default `apiVersion` used for fetching/updating
ingresses from `extensions/v1beta1` to `networking.k8s.io/v1beta1`. For this to
work the controller needs to have permissions to `list` `ingresses` and
`update`, `patch` `ingresses/status` from the `networking.k8s.io` `apiGroup`.
[See deployment example](deploy/ingress-serviceaccount.yaml). To fallback to
the old behavior you can set the apiVersion via the `--ingress-api-version`
flag. Value must be `extensions/v1beta1` or `networking.k8s.io/v1beta1`
(default) or `networking.k8s.io/v1`.

### <v0.9.0 to >=v0.9.0

Version `v0.9.0` changes the internal flag parsing library to
[kingpin][kingpin] this means flags are now defined with `--` (two dashes)
instead of a single dash. You need to change all the flags like this:
`-stack-termination-protection` -> `--stack-termination-protection` before
running `v0.9.0` of the controller.

[kingpin]: https://github.com/alecthomas/kingpin

### <v0.8.0 to >=v0.8.0

Version `v0.8.0` added certificate verification check to automatically ignore
self-signed and certificates from internal CAs. The IAM role used by the controller
now needs the `acm:GetCertificate` permission. `acm:DescribeCertificate` permission
is no longer needed and can be removed from the role.

### <v0.7.0 to >=v0.7.0

Version `v0.7.0` deletes the annotation
`zalando.org/aws-load-balancer-ssl-cert-domain`, which we do not
consider as feature since we have SNI enabled ALBs.

### <v0.6.0 to >=v0.6.0

Version `v0.6.0` introduced support for Multiple TLS Certificates per ALB
(SNI). When upgrading your ALBs will automatically be aggregated to a single
ALB with multiple certificates configured.
It also adds support for attaching single EC2 instances and multiple
AutoScalingGroups to the ALBs therefore you must ensure you have the correct
instance filter defined before upgrading. The default filter is
`tag:kubernetes.io/cluster/<cluster-id>=owned tag-key=k8s.io/role/node` see
[How it works](#how-it-works) for more information on how to configure this.

### <v0.5.0 to >=v0.5.0

Version `v0.5.0` introduced support for both `internet-facing` and `internal`
load balancers. For this change we had to change the naming of the
CloudFormation stacks created by the controller. To upgrade from v0.4.* to
v0.5.0 no changes are needed, but since the naming change of the stacks
migrating back down to a v0.4.* version will not be non-disruptive as it will
be unable to manage the stacks with the new naming scheme. Deleting the stacks
manually will allow for a working downgrade.

### <v0.4.0 to >=v0.4.0

In versions before v0.4.0 we used AWS Tags that were set by CloudFormation automatically to find
some AWS resources.
This behavior has been changed to use custom non cloudformation tags.

In order to update to v0.4.0, you have to add the following tags to your AWs Loadbalancer
SecurityGroup before updating:
- `kubernetes:application=kube-ingress-aws-controller`
- `kubernetes.io/cluster/<cluster-id>=owned`

Additionally you must ensure that the instance where the ingress-controller is
running has the clusterID tag `kubernetes.io/cluster/<cluster-id>=owned` set
(was `ClusterID=<cluster-id>` before v0.4.0).

## Ingress annotations

Overview of configuration which can be set via Ingress annotations.

### Annotations
|Name                       | Value |Default
|---------------------------|------|------|
|[`alb.ingress.kubernetes.io/ip-address-type`](#ip-address-type)|`ipv4` \| `dualstack` |`ipv4`|
|`zalando.org/aws-load-balancer-ssl-cert`|`string`|N/A|
|`zalando.org/aws-load-balancer-scheme`|`internal` \| `internet-facing` |`internet-facing`|
|`zalando.org/aws-load-balancer-shared`|`true` \| `false`|`true`|
|`zalando.org/aws-load-balancer-security-group`|`string`|N/A|
|`zalando.org/aws-load-balancer-ssl-policy`|`string`|`ELBSecurityPolicy-2016-08`|
|`zalando.org/aws-load-balancer-type`| `nlb` \| `alb`|`alb`|
|`zalando.org/aws-load-balancer-http2`| `true` \| `false`|`true`|
|`zalando.org/aws-waf-web-acl-id` | `string` | N/A |
|`kubernetes.io/ingress.class`|`string`|N/A|

The defaults can also be configured globally via a flag on the controller.

Note that the annotation `alb.ingress.kubernetes.io/ip-address-type` can be used for both ALB and NLB. We use the same annotation for compatibility with external DNS. The `kubernetes-sigs/external-dns` project (see [PR #1079](https://github.com/kubernetes-sigs/external-dns/pull/1079)) only recognizes this specific annotation for DNS record management. If we were to change the annotation, external-dns would not adopt it, resulting in missing or incorrect DNS records. This constraint was discussed in the [Kubernetes Slack channel](https://kubernetes.slack.com/archives/C0LRMHZ1T/p1764951540935419), and changing it is not considered necessary at this time.

## Load Balancers types

The controller supports both [Application Load Balancers][alb] and [Network
Load Balancers][nlb]. Below is an overview of which features can be used with
the individual Load Balancer types.

| Feature                                 | Application Load Balancer                      | Network Load Balancer                    |
|-----------------------------------------|------------------------------------------------|------------------------------------------|
| HTTPS                                   | :heavy_check_mark:                             | :heavy_check_mark:                       |
| HTTP                                    | :heavy_check_mark:                             | :heavy_check_mark: `--nlb-http-enabled`  |
| HTTP -> HTTPS redirect                  | :heavy_check_mark: `--redirect-http-to-https`  | :heavy_multiplication_x:                 |
| [Cross Zone Load Balancing][cross_zone] | :heavy_check_mark: (only option)               | :heavy_check_mark: `--nlb-cross-zone`    |
| [Zone Affinity][zone_affinity]          | :heavy_multiplication_x:                       | :heavy_check_mark: `--nlb-zone-affinity` |
| [Dualstack support][dualstack]          | :heavy_check_mark: `--ip-addr-type=dualstack`  | :heavy_multiplication_x:                 |
| [Idle Timeout][idle_timeout]            | :heavy_check_mark: `--idle-connection-timeout` | :heavy_multiplication_x:                 |
| Custom Security Group                   | :heavy_check_mark:                             | :heavy_multiplication_x:                 |
| Web Application Firewall (WAF)          | :heavy_check_mark:                             | :heavy_multiplication_x:                 |
| HTTP/2 Support                          | :white_check_mark:                             | (not relevant)                           |

To facilitate default load balancer type switch from Application to Network when the default load balancer type is Network
(`--load-balancer-type="network"`) and Custom Security Group (`zalando.org/aws-load-balancer-security-group`) or
Web Application Firewall (`zalando.org/aws-waf-web-acl-id`) annotation is present the controller configures Application Load Balancer.
If `zalando.org/aws-load-balancer-type: nlb` annotation is also present then controller ignores the configuration and logs an error.

[cross_zone]: https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html#availability-zones
[zone_affinity]: https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html#zonal-dns-affinity
[dualstack]: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/application-load-balancers.html#ip-address-type
[idle_timeout]: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/application-load-balancers.html#load-balancer-attributes

## AWS Tags

SecurityGroup auto detection needs the following AWS Tags on the
SecurityGroup:
- `kubernetes.io/cluster/<cluster-id>=owned`
- `kubernetes:application=<controller-id>`, controller-id defaults to
`kube-ingress-aws-controller` and can be set by flag `--controller-id=<my-ctrl-id>`.

AutoScalingGroup auto detection needs the same AWS tags  on the
AutoScalingGroup as defined for the SecurityGroup.

In case you want to attach/detach single EC2 instances to the ALB
TargetGroup, you have to have the same `<cluster-id>` set as on the
running kube-ingress-aws-controller. Normally this would be
`kubernetes.io/cluster/<cluster-id>=owned`.

## Development Status

This controller is used in production since Q1 2017. It aims to be out-of-the-box useful for anyone
running Kubernetes. Jump down to the [Quickstart](#trying-it-out) to try it out—and please let us know if you have
trouble getting it running by filing an
[Issue](https://github.com/zalando-incubator/kube-ingress-aws-controller/issues).
If you created your cluster with [Kops](https://github.com/kubernetes/kops), see our [deployment guide for Kops](deploy/kops.md)

As of this writing, it's being used in production use cases at [Zalando](https://tech.zalando.com/), and can be considered battle-tested in this setup. We're actively seeking devs/teams/companies to try it out and share feedback so we can
make improvements.

We are also eager to bring new contributors on board. See [our contributor guidelines](CONTRIBUTING.md)
to get started, or [claim a "Help Wanted" item](https://github.com/zalando-incubator/kube-ingress-aws-controller/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22).

## Why We Created This Ingress Controller

The maintainers of this project are building an infrastructure that runs [Kubernetes on top of AWS](https://github.com/zalando-incubator/kubernetes-on-aws) at large scale (for nearly 200 delivery teams), and with automation. As such, we're creating our own tooling to support this new infrastructure. We couldn't find an existing ingress controller that operates like this one does, so we created one ourselves.

We're using this ingress controller with [Skipper](http://github.com/zalando/skipper), an HTTP router that Zalando
has used in production since Q4 2015 as part of its front-end microservices architecture. Skipper's also open
source and has some outstanding features, that we
[documented here](https://opensource.zalando.com/skipper/kubernetes/ingress-controller/). Feel
free to use it, or use another ingress of your choosing.


## How It Works

This controller continuously polls the API server to check for ingress resources. It runs an infinite loop. For
each cycle it creates load balancers for new ingress resources, and deletes the load balancers for obsolete/removed
ingress resources.

This is achieved using AWS CloudFormation. For more details check our [CloudFormation Documentation](cloudformation.md)

The controller *will not* manage the security groups required to allow access from the Internet to the load balancers.
It assumes that their lifecycle is external to the controller itself.

During startup phase EC2 filters are constructed as follows:

* If `CUSTOM_FILTERS` environment variable is set, it is used to generate filters that are later used
  to fetch instances from EC2.
* If `CUSTOM_FILTERS` environment variable is not set or could not be parsed, then default
  filters are `tag:kubernetes.io/cluster/<cluster-id>=owned tag-key=k8s.io/role/node` where `<cluster-id>`
  is determined from EC2 tags of instance on which Ingress Controller pod is started.

`CUSTOM_FILTERS` is a list of filters separated by spaces. Each filter has a form of `name=value` where name can be a `tag:` or `tag-key:` prefixed expression, as would be recognized by the EC2 API, and value is value of a filter, or a comma seperated list of values.

For example:

* `tag-key=test` will filter instances that have a tag named `test`, ignoring the value.
* `tag:foo=bar'` will filter instances that have a tag named `foo` with the value `bar`
* `tag:abc=def,ghi` will filter instances that have a tag named `abc` with the value `def` OR `ghi`
* Default filter `tag:kubernetes.io/cluster/<cluster-id>=owned tag-key=k8s.io/role/node` filters instances
  that has tag `kubernetes.io/cluster/<cluster-id>` with value `owned` and have tag named `tag-key=k8s.io/role/node`.

Every poll cycle EC2 is queried with filters that were constructed during startup.
Each new discovered instance is scanned for Auto Scaling Group tag. Each Target
Group created by this Ingress controller is then added to each known Auto Scaling Group.
Each Auto Scaling Group information is fetched only once when first node of it is discovered for first time.
If instance does not belong to Auto Scaling Group (does not have `aws:autoscaling:groupName` tag) it is stored in separate list of
Single Instances. On each cycle instances on this list are registered as targets in all Target Groups managed by this controller.
If call to get instances from EC2 did not return previously known Single Instance, it is deregistered from Target Group and removed from list of Single Instances.
Call to deregister instances is aggregated so that maximum 1 call to deregister is issued in poll cycle.

For Auto Scaling Groups, the controller will always try to build a list of
*owned* Auto Scaling Groups based on the tag:
`kubernetes.io/cluster/<cluster-id>=owned` even if this tag is not specified in
the `CUSTOM_FILTERS` configuration. Tracking the *owned* Auto Scaling Groups is
done to automatically deregister any ASGs which are no longer targeted by the
`CUSTOM_FILTERS`.

### Discovery

On startup, the controller discovers the AWS resources required for the controller operations:

1. The Security Group

    Lookup of the `kubernetes.io/cluster/<cluster-id>` tag of the Security Group matching the clusterID for the controller node and `kubernetes:application` matching the value `kube-ingress-aws-controller` or as fallback for `<v0.4.0`
    tag `aws:cloudformation:logical-id` matching the value `IngressLoadBalancerSecurityGroup` (only clusters created by CF).

2. The Subnets

    Subnets are discovered based on the VPC of the instance where the
    controller is running. By default it will try to select all subnets of the
    VPC but will limit the subnets to one per Availability Zone. If there are
    many subnets within the VPC it's possible to tag the desired subnets with
    the tags `kubernetes.io/role/elb` (for internet-facing ALBs) or
    `kubernetes.io/role/internal-elb` (for internal ALBs). Subnets with these
    tags will be favored when selecting subnets for the ALBs.
    Additionally you can tag EC2 subnets with
    `kubernetes.io/cluster/<cluster-id>`, which will be prioritized.
    If there are two possible subnets for a single Availability Zone then the
    first subnet, lexicographically sorted by ID, will be selected.

#### Running outside of EC2

The controller can run outside of EC2. In this mode it can't dicover `vpc-id`
and `cluster-id` which needs to be passed via flags on startup:

```sh
./kube-ingress-aws-controller \
  --cluster-id="<cluster-id>" \
  --vpc-id="<vpc-id>"
```

You can get the VPC ID by listing VPCs in your AWS account:

```sh
aws ec2 describe-vpcs
{
    "Vpcs": [
        {
            "CidrBlock": "172.31.0.0/16",
            "DhcpOptionsId": "....",
            "State": "available",
            "VpcId": "vpc-abcde",
            ...
```

### Creating Load Balancers

When the controller learns about new ingress resources, it uses the hosts specified in it to automatically determine
the most specific, valid certificates to use. The certificates has to be valid for at least 7 days. An example ingress:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-app
spec:
  rules:
  - host: test-app.example.org
    http:
      paths:
      - backend:
          service:
            name: test-app-service
            port:
              name: main-port
        path: /
        pathType: ImplementationSpecific
```

The Application Load Balancer created by the controller will have both an HTTP listener and an HTTPS listener. The
latter will use the automatically selected certificates.

By default the ingress-controller will aggregate all ingresses under as few
Application Load Balancers as possible (unless running with
`--disable-sni-support`). If you like to provision an Application Load Balancer
that is unique for an ingress you can use the annotation
`zalando.org/aws-load-balancer-shared: "false"`.

The new Application Load Balancers have a custom tag marking them as *managed* load balancers to differentiate them
from other load balancers. The tag looks like this:

    `kubernetes:application` = `kube-ingress-aws-controller`

They also share the `kubernetes.io/cluster/<cluster-id>` tag with other resources from the cluster where it belongs.

#### Create a Load Balancer with a pinned certificate

As a second option you can specify the [Amazon Resource Name](https://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html) (ARN)
of the desired certificate with an annotation like the one shown here:

```yaml
apiVersion: networking.k8s.io/v1
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
          service:
            name: test-app-service
            port:
              name: main-port
        path: /
        pathType: ImplementationSpecific
```

#### Create an internal Load Balancer

You can select the [Application Load Balancer Scheme](http://docs.aws.amazon.com/elasticloadbalancing/latest/userguide/how-elastic-load-balancing-works.html#load-balancer-scheme)
with an annotation like the one shown here:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myingress
  annotations:
    zalando.org/aws-load-balancer-scheme: internal
spec:
  rules:
  - host: test-app.example.org
    http:
      paths:
      - backend:
          service:
            name: test-app-service
            port:
              name: main-port
        path: /
        pathType: ImplementationSpecific
```

You can only select from `internet-facing` (default) and `internal`
options.

If you run the controller with `--load-balancer-type=network` and
create an `internal` load balancer, the controller will create an
Application Load Balancer instead of a Network Load Balancer, because
it can create [hard to debug issues](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-troubleshooting.html#intermittent-connection-failure),
that we want to prevent as default. If you know what you are doing you
can enforce to create a Network Load Balancer by setting annotation
`zalando.org/aws-load-balancer-type: nlb`.


#### Omit to create a Load Balancer for cluster internal domains

Since `>=v0.10.5`, you can create Ingress objects with `host` rules,
that have the `.cluster.local` and the controller will not create an
ALB for this.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myingress
spec:
  rules:
  - host: test-app.skipper.cluster.local
    http:
      paths:
      - backend:
          service:
            name: test-app-service
            port:
              name: main-port
        path: /
        pathType: ImplementationSpecific
```

If you pass `--cluster-local-domain=".cluster.local"`, you can change
what domain is considered cluster internal. If you're using the [deny
internal traffic](#deny-traffic-for-internal-domains) feature, you might
want to sync this configuration with the `--internal-domains` one.

#### Deny traffic for internal domains

Since `>=v0.11.18` the controller supports the flag
`--deny-internal-domains`. It's a boolean config item that when enabled
configures the ALBs' cloudformation templates with a
[`AWS::ElasticLoadBalancingV2::ListenerRule`][ListenerRule] resource.
This rule will be configured with the [condition][HostHeaderConfig]
values from the `--internal-domains` flag and the
[action `fixedresponseconfig`][FixedResponse] with the respective response
`--deny-internal-domains-response` flags. This feature is not enabled by
default. The following are the default values to its config flags:

- `internal-domains`: `*.cluster.local`
- `deny-internal-domains`: `false` (same as explicitly passing
  `--no-deny-internal-domains`)
- `deny-internal-domains-response`: `Unauthorized`
- `deny-internal-domains-response-content-type`: `text/plain`
- `deny-internal-domains-response-status-code`: `401`

Note that `--internal-domains` differs from `--cluster-local-domain`,
which is used exclusively to [avoid load balancers creation for the
cluster internal
domain](#omit-to-create-a-load-balancer-for-cluster-internal-domains).
The `--internal-domains` flag can be set multiple times and accept AWS'
wildcard characters. Check the AWS' docs on the [Host Header
config][HostHeaderConfig] for more details.

This feature is not supported by NLBs.

Example:

Running the controller with `--deny-internal-domains` and
`--internal-domains=*.cluster.local` will generate a rule in the ALB
that matches any request to domains ending in `.cluster.local` and answer
the request with an [HTTP 401 Unauthorized][401].

[ListenerRule]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-listenerrule.html
[HostHeaderConfig]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-hostheaderconfig.html
[FixedResponse]: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listenerrule-action.html#cfn-elasticloadbalancingv2-listenerrule-action-fixedresponseconfig
[401]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/401

#### Create Load Balancer with SSL Policy

You can select the default
[SSLPolicy](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/create-https-listener.html#describe-ssl-policies),
with the flag `--ssl-policy=ELBSecurityPolicy-TLS-1-2-2017-01`. This
choice can be overriden by the Kubernetes Ingress annotation
`zalando.org/aws-load-balancer-ssl-policy` to any valid value. Valid
values will be checked by the controller.

Example:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myingress
  annotations:
    zalando.org/aws-load-balancer-ssl-policy: ELBSecurityPolicy-FS-2018-06
spec:
  rules:
  - host: test-app.example.org
    http:
      paths:
      - backend:
          service:
            name: test-app-service
            port:
              name: main-port
        path: /
        pathType: ImplementationSpecific
```

#### Create Load Balancer with SecurityGroup

The controller will normally automatically detect the SecurityGroup to
use. Auto detection is done by filtering all SecurityGroups with AWS
Tags. The `kubernetes.io/cluster/<cluster-id>` tag of the Security
Group should match clusterID for the controller node with value
`owned` and `kubernetes:application` tag should match the value
`kube-ingress-aws-controller`.

If you want to override the detected SecurityGroup, you can set a
SecurityGroup of your choice with the
`zalando.org/aws-load-balancer-security-group` annotation like the
shown here:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myingress
  annotations:
    zalando.org/aws-load-balancer-security-group: sg-somegroupeid
spec:
  rules:
  - host: test-app.example.org
    http:
      paths:
      - backend:
          service:
            name: test-app-service
            port:
              name: main-port
        path: /
        pathType: ImplementationSpecific
```

#### Create Load Balancers with WAF associations

It is possible to define WAF associations for the created load balancers. The WAF Web ACLs need to be created
separately via CloudFormation or the AWS Console, and they can be referenced either as a global startup
configuration of the controller, or as ingress specific settings in the ingress object with an annotation. The
ingress annotation overrides the global setting, and the controller will create separate load balancers for
those ingresses using a separate WAF association.

The controller supports two versions of AWS WAF:

- WAF (v1 or "classic"): the Web ACL is identified by a UUID
- WAFv2: the Web ACL is identified by its ARN, prefixed with `arn:aws:wafv2:`

Only one WAF association can be used for a load balancer, and the same command line flag and ingress annotation
is used for both versions, only the format of the value differs.

##### Starting the controller with global WAF association:

```
kube-ingress-aws-controller --aws-waf-web-acl-id=arn:aws:wafv2:eu-central-1:123456789012:regional/webacl/test-waf-acl/12345678-abcd-efgh-ijkl-901234567890
```

##### Setting ingress specicif WAF association:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myingress
  annotations:
    zalando.org/aws-waf-web-acl-id: arn:aws:wafv2:eu-central-1:123456789012:regional/webacl/test-waf-acl/12345678-abcd-efgh-ijkl-901234567890
spec:
  rules:
  - host: test-app.example.org
    http:
      paths:
      - backend:
          service:
            name: test-app-service
            port:
              name: main-port
        path: /
        pathType: ImplementationSpecific
```



### Deleting load balancers

When the controller detects that a managed load balancer for the current cluster doesn't have a matching ingress
resource anymore, it deletes all the previously created resources.

Deletion may take up to about 30 minutes. This ensures proper draining of connections on the loadbalancers and allows for DNS TTLs to expire.

## Building

This project provides a [`Makefile`](https://github.com/zalando-incubator/kube-ingress-aws-controller/blob/master/Makefile)
that you can use to build either a binary or a Docker image.

### Building a Binary

To build a binary for the Linux operating system, simply run `make` or `make build.linux`.

### Building a Docker Image

To create a Docker image instead, execute `make build.docker`. You can then push your Docker image to the Docker
registry of your choice.

## Deploy

To [deploy](deploy/README.md) the ingress controller, use the
[example YAML](deploy/ingress-controller.yaml.example) as the descriptor.
You can customize the image used in the example YAML file.

We provide `ghcr.io/zalando-incubator/kube-ingress-aws-controller:latest` as a publicly usable Docker image
built from this codebase. You can deploy it with 2 easy steps:
- Replace the placeholder for your region inside the example YAML, for ex., `eu-west-1`
- Use kubectl to execute the command  `kubectl apply -f deploy/ingress-controller.yaml`

If you use [Kops](https://github.com/kubernetes/kops) to create your
cluster, please use our [deployment guide for Kops](deploy/kops.md)

## Running multiple instances

In some cases it might be useful to run multiple instances of this controller:

* Isolating internal vs external traffic
* Using a different set of traffic processing nodes
* Using different frontend routers (e.g.: Skipper and Traefik)

You can use the flag `--controller-id` to set a token that will be used to isolate resources between controller instances.
This value will be used to tag those resources.

If you don't pass an ID, the default `kube-ingress-aws-controller` will be used.

Usually you would want to combine this flag with `ingress-class-filter` so different types of ingresses are associated with the different controllers.
To make `kube-ingress-aws-controller` manage both specific ingress class and an empty one (or ingresses without ingress class annotation) add an empty class to the list. For example to manage ingress class `foo` and ingresses without class set parameter like this `--ingress-class-filter=foo,` (notice the comma in the end).

Ingress classes defined in the spec of ingresses at `spec.ingressClassName` ([Kubernetes Documentation](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/ingress-v1/#IngressSpec)) will take priority over the annotation, if both are supplied.
In order to match the default (empty) ingress group, both must be empty."

## Target and Health Check Ports

By default the port 9999 is used as both health check and target port. This
means that Skipper or any other traffic router you're using needs to be
listening on that port.

If you want to change the default ports, you can control it using the
`-target-port` and `-health-check-port` flags.

If you want to use an HTTPS enabled target port, use the `-target-https` flag.
This will only affect ALBs, NLBs ignore this flag.

## HTTP to HTTPS Redirection

By default, the controller will expose both HTTP and HTTPS ports on the load balancer, and forward both listeners to the target port. Setting the flag `-redirect-http-to-https` will instead configure the HTTP listener to emit a 301 redirect for any request received, with the destination location being the same URL but with the HTTPS scheme vs. HTTP. The specifics are described in the [relevant aws documentation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-elasticloadbalancingv2-listener-redirectconfig.html).

### Backward Compatibility

The controller used to have only the `--health-check-port` flag available, and would use the same port as health check and the target port.
Those ports are now configured individually. If you relied on this behavior, please include the `--target-port` in your configuration.

## Zone Aware Traffic

If you want to have full zone aware traffic from client to the NLB target members, you can configure the controller by 2 configuration parameters:

1. [Zone Affinity][zone_affinity] to resolve DNS via Route53 to the same zone NLB Listener `--nlb-zone-affinity=availability_zone_affinity`
2. [Cross Zone Load Balancing][cross_zone] to disable cross zone balancing from NLB to member  `--nlb-cross-zone=false`

[Zone Affinity][zone_affinity] has 3 options:

1. `availability_zone_affinity`: 100% zonal affinity
2. `partial_availability_zone_affinity`: 85% zonal affinity
3. `any_availability_zone`: 0% zonal affinity

The default is to run with cross zone traffic enabled and any zone affinity.

## AWS CNI Mode (experimental)

The common operation mode of the controller (`--target-access-mode=HostPort`) is to link the target groups to the autoscaling group.
The target group type is `instance`, requiring the ingress pod to be accessible through a `HostNetwork` and `HostPort`.

In *AWS CNI Mode* (`--target-access-mode=AWSCNI`) the controller actively manages the target group members.
Since AWS EKS cluster running AWS VPC CNI have their pods as first class members in the VPCs, they can receive the traffic directly,
being managed through a target group type is `ip`, which means there is no necessity for the HostPort indirection.

### Notes

- For security reasons the HostPort requirement might be of concern
- Direct management of the target group members is significantly faster compared to the AWS linked mode, but it requires
  a running controller for updates. As of now, the controller is not prepared for high availability replicated setup.
- The registration and deregistration is synced with the pod lifecycle, hence a pod in terminating phase is deregistered
  from the target group before shut down.
- Ingress pods are not bound to nodes in CNI mode and the deployment can scale independently.

### Configuration options

| access mode | HostNetwork | HostPort |                      Notes                             |
| :---------: | :---------: | :------: | :----------------------------------------------------: |
| `HostPort`  |   `true`    |  `true`  | target group updated by ASG, see v0.14.0 release notes |
| `AWSCNI`    |   `true`    |  `true`  | PodIP == HostIP: limited scaling and host bound        |
| `AWSCNI`    |   `false`   |  `true`  | PodIP != HostIP: limited scaling and host bound        |
| `AWSCNI`    |   `false`   | `false`  | free scaling, pod VPC CNI IP used                      |

## Trying it out

The Ingress Controller's responsibility is limited to managing load balancers, as described above. To have a fully
functional setup, additionally to the ingress controller, you can use [Skipper](https://github.com/zalando/skipper)
to route the traffic to the application. The setup follows what's
described [here](https://kubernetes-on-aws.readthedocs.io/en/latest/user-guide/ingress.html).

You can deploy `skipper` as a `DaemonSet` using [another example YAML](deploy/skipper.yaml) by executing the following command:

```
kubectl apply -f deploy/skipper.yaml
```

To complete the setup, you'll need to fulfill some additional requirements regarding security groups and IAM
roles; [more info here](deploy/requirements.md).

### DNS

To have convenient DNS names for your application, you can use the
Kubernetes-Incubator project, [external-dns](https://github.com/kubernetes-incubator/external-dns).
It's not strictly necessary for this Ingress Controller to work,
though.

## Contributing

We welcome your contributions, ideas and bug reports via issues and pull requests;
[here are those Contributor guidelines again](CONTRIBUTING.md).

## Contact

Check our [MAINTAINERS file](MAINTAINERS) for email addresses.

## Security

We welcome your security reports please checkout our
[SECURITY.md](SECURITY.md).

## License

The MIT License (MIT) Copyright © [2017] Zalando SE, https://tech.zalando.com

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.




[alb]: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/introduction.html
[nlb]: https://docs.aws.amazon.com/elasticloadbalancing/latest/network/introduction.html
