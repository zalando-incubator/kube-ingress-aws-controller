# Deployment guide using Kops and Skipper (http router)

[Kube AWS Ingress Controller](https://github.com/zalando-incubator/kubernetes-on-aws)
creates AWS Application Load Balancer (ALB) that is used to terminate TLS connections and use
[AWS Certificate Manager (ACM)](https://aws.amazon.com/certificate-manager/) or
[AWS Identity and Access Management (IAM)](http://docs.aws.amazon.com/IAM/latest/APIReference/Welcome.html)
certificates. ALBs are used to route traffic to an Ingress http router for example
[skipper](https://github.com/zalando/skipper/), which routes
traffic to Kubernetes services and implements
[advanced features](https://zalando.github.io/skipper/dataclients/kubernetes/)
like green-blue deployments, feature toggles, reate limits,
circuitbreakers, opentracing API, shadow traffic or A/B tests.

In short the major differences from CoreOS ALB Ingress Controller is:

- it uses Cloudformation instead of API calls
- does not have routes limitations from AWS
- automatically finds the best matching ACM and IAM certifacte for your ingress
- you are free to use an http router imlementation of your choice, which can implement more features like green-blue deployments

For this tutorial I assume you have GNU sed installed, if not read
commands with `sed` to modify the files according to the `sed` command
being run. If you are running BSD or MacOS you can use `gsed`.

## Create Kops cluster with cloud labels

Cloud Labels are required to make Kube AWS Ingress Controller work,
because it has to find the AWS Application Load Balancers it manages
by AWS Tags, which are called cloud Labels in Kops.

You have to set some environment variables to choose AZs to deploy to,
your S3 Bucket name for Kops configuration and you Kops cluster name:


        export AWS_AVAILABILITY_ZONES=eu-central-1b,eu-central-1c
        export S3_BUCKET=kops-aws-workshop-<your-name>
        export KOPS_CLUSTER_NAME=example.cluster.k8s.local


Next, you create the Kops cluster and validate that everything is set up properly.


        export KOPS_STATE_STORE=s3://${S3_BUCKET}
        kops create cluster --name $KOPS_CLUSTER_NAME --zones $AWS_AVAILABILITY_ZONES --cloud-labels kubernetes.io/cluster/$KOPS_CLUSTER_NAME=owned --yes
        kops validate cluster


### IAM role

This is the effective policy that you need for your EC2 nodes for the
kube-aws-ingress-controller, which we will use:

```
{
  "Effect": "Allow",
  "Action": [
    "acm:ListCertificates",
    "acm:DescribeCertificate",
    "autoscaling:DescribeAutoScalingGroups",
    "autoscaling:AttachLoadBalancers",
    "autoscaling:DetachLoadBalancers",
    "autoscaling:DetachLoadBalancerTargetGroups",
    "autoscaling:AttachLoadBalancerTargetGroups",
    "cloudformation:*",
    "elasticloadbalancing:*",
    "elasticloadbalancingv2:*",
    "ec2:DescribeInstances",
    "ec2:DescribeSubnets",
    "ec2:DescribeSecurityGroups",
    "ec2:DescribeRouteTables",
    "ec2:DescribeVpcs",
    "iam:GetServerCertificate",
    "iam:ListServerCertificates"
  ],
  "Resource": [
    "*"
  ]
}
```

To apply the mentioned policy you have to add [additionalPolicies with kops](https://github.com/kubernetes/kops/blob/master/docs/iam_roles.md) for your cluster, so edit your cluster.

        kops edit cluster $KOPS_CLUSTER_NAME

and add this to your node policy:

```
  additionalPolicies:
    node: |
      [
        {
          "Effect": "Allow",
          "Action": [
            "acm:ListCertificates",
            "acm:DescribeCertificate",
            "autoscaling:DescribeAutoScalingGroups",
            "autoscaling:AttachLoadBalancers",
            "autoscaling:DetachLoadBalancers",
            "autoscaling:DetachLoadBalancerTargetGroups",
            "autoscaling:AttachLoadBalancerTargetGroups",
            "cloudformation:*",
            "elasticloadbalancing:*",
            "elasticloadbalancingv2:*",
            "ec2:DescribeInstances",
            "ec2:DescribeSubnets",
            "ec2:DescribeSecurityGroups",
            "ec2:DescribeRouteTables",
            "ec2:DescribeVpcs",
            "iam:GetServerCertificate",
            "iam:ListServerCertificates"
          ],
          "Resource": ["*"]
        }
      ]
```

After that make sure this was applied to your cluster with:


        kops update cluster $KOPS_CLUSTER_NAME --yes
        kops rolling-update cluster


### Security Group for Ingress

To be able to route traffic from ALB to your nodes you need to create
an Amazon EC2 security group with Kubernetes tags, that allow ingress
port 80 and 443 from the internet and everything from ALBs to your
nodes. Tags are used from Kubernetes components to find AWS components
owned by the cluster. We will do with the AWS cli:

        aws ec2 create-security-group --description ingress.$KOPS_CLUSTER_NAME --group-name ingress.$KOPS_CLUSTER_NAME
        aws ec2 describe-security-groups --group-names ingress.$KOPS_CLUSTER_NAME
        sgidingress=$(aws ec2 describe-security-groups --filters Name=group-name,Values=ingress.$KOPS_CLUSTER_NAME | jq '.["SecurityGroups"][0]["GroupId"]' -r)
        sgidnode=$(aws ec2 describe-security-groups --filters Name=group-name,Values=nodes.$KOPS_CLUSTER_NAME | jq '.["SecurityGroups"][0]["GroupId"]' -r)
        aws ec2 authorize-security-group-ingress --group-id $sgidingress --protocol tcp --port 443 --cidr 0.0.0.0/0
        aws ec2 authorize-security-group-ingress --group-id $sgidingress --protocol tcp --port 80 --cidr 0.0.0.0/0

        aws ec2 authorize-security-group-ingress --group-id $sgidnode --protocol all --port -1 --source-group $sgidingress
        aws ec2 create-tags --resources $sgidingress--tags "kubernetes.io/cluster/id=owned" "kubernetes:application=kube-ingress-aws-controller"

### AWS Certificate Manager (ACM)

To have TLS termination you can use AWS managed certificates.  If you
are unsure if you have at least one certificate provisioned use the
following command to list ACM certificates:

        aws acm list-certificates

If you have one, you can move on to the next section.

To create an ACM certificate, you have to requset a CSR with a domain name that you own in [route53](https://aws.amazon.com/route53/), for example.org. We will here request one wildcard certificate for example.org:

        aws acm request-certificate --domain-name *.example.org

You will have to successfully do a challenge to show ownership of the
given domain. In most cases you have to click on a link from an e-mail
sent by certificates.amazon.com. E-Mail subject will be `Certificate approval for <example.org>`.

If you did the challenge successfully, you can now check the status of
your certificate. Find the ARN of the new certificate:

        aws acm list-certificates

Describe the certificate and check the Status value:

        aws acm describe-certificate --certificate-arn arn:aws:acm:<snip> | jq '.["Certificate"]["Status"]'

If this is no "ISSUED", your certificate is not valid and you have to fix it.
To resend the CSR validation e-mail, you can use

        aws acm resend-validation-email


### Install components kube-aws-ingress-controller and skipper

Kube-aws-ingress-controller will be deployed as deployment with 1
replica, which is ok for production, because it's only configuring
ALBs.

        REGION=${AWS_AVAILABILITY_ZONES#*,}
        REGION=${REGION:0:-1}
        sed -i "s/<REGION>/$REGION/" deploy/ingress-controller.yaml
        kubectl create -f deploy/ingress-controller.yaml

Skipper will be deployed as daemonset:

        kubectl create -f deploy/skipper.yaml

Check, if the installation was successful:

        kops validate cluster

If not and you are sure all steps before were done, please check the logs of the POD, which is not in running state:

        kubectl -n kube-system get pods -l component=ingress
        kubectl -n kube-system logs <podname>

### Test deployment
#### Base features

Deploy one sample application and change the hostname depending on
your route53 domain and ACM certificate:

        kubectl create -f deploy/examples/sample-app-v1.yaml
        kubectl create -f deploy/examples/sample-svc-v1.yaml
        sed -i "s/<HOSTNAME>/demo-app.example.org/" deploy/examples/sample-ing-v1.yaml
        kubectl create -f deploy/examples/sample-ing-v1.yaml

Check if your deployment was successful:

        kubectl get pods,svc -l application=demo

To check if your Ingress created an ALB check the `ADDRESS` column:

        kubectl get ing -l application=demo -o wide
        NAME           HOSTS                          ADDRESS                                                              PORTS     AGE
        demo-app-v1   myapp.example.org   example-lb-19tamgwi3atjf-1066321195.us-central-1.elb.amazonaws.com   80        1m

If it is provisioned you can check with curl, http to https redirect is created automatically by Skipper:

        curl -L -H"Host: myapp.example.org" example-lb-19tamgwi3atjf-1066321195.us-central-1.elb.amazonaws.com
        <body style='color: green; background-color: white;'><h1>Hello!</h1>

Check if Kops dns-controller created a DNS record:

        curl -L myapp.example.org
        <body style='color: green; background-color: white;'><h1>Hello!</h1>

#### Advanced Features

We assume you have all components running that were applied in `Base features`.

Deploy a second ingress with a feature toggle and rate limit to protect you backend:

        sed -i "s/<HOSTNAME>/demo-app.example.org/" deploy/examples/sample-ing-v2.yaml
        kubectl create -f deploy/examples/sample-ing-v2.yaml

Deploy a second sample application:

        kubectl create -f deploy/examples/sample-app-v2.yaml
        kubectl create -f deploy/examples/sample-svc-v2.yaml

Now, you can test the feature toggle to access the new v2 application:

        curl "https://myapp.example.org/?version=v2"
        <body style='color: white; background-color: green;'><h1>Hello AWS!</h1>

If you run this more often, you can easily trigger the rate limit to stop proxying your call to the backend:

        for i in {0..9}; do curl -v "https://myapp.example.org/?version=v2"; done

You should see output similar to:

        *   Trying 52.222.161.4...
        -------- a lot of TLS output --------
        > GET /?version=v2 HTTP/1.1
        > Host: myapp.example.org
        > User-Agent: curl/7.49.0
        > Accept: */*
        >
        < HTTP/1.1 429 Too Many Requests
        < Content-Type: text/plain; charset=utf-8
        < Server: Skipper
        < X-Content-Type-Options: nosniff
        < X-Rate-Limit: 60
        < Date: Mon, 27 Nov 2017 18:19:26 GMT
        < Content-Length: 18
        <
        Too Many Requests
        * Connection #0 to host myapp.example.org left intact

Your endpoint is now protected.

Next we will show traffic switching.
Deploy an ingress with traffic switching 80% traffic goes to v1 and
20% to v2. Change the hostname depending on your route53 domain and
ACM certificate as before:

        sed -i "s/<HOSTNAME>/demo-app.example.org/" deploy/examples/sample-ing-tf.yaml
        kubectl create -f deploy/examples/sample-ing-traffic.yaml

Remove old ingress which will interfere with the new created one:

        kubectl delete -f deploy/examples/sample-ing-v1.yaml
        kubectl delete -f deploy/examples/sample-ing-v2.yaml

Check deployments and services (both should be 2)

        kubectl get pods,svc -l application=demo

To check if your Ingress has an ALB check the `ADDRESS` column:

        kubectl get ing -l application=demo -o wide
        NAME           HOSTS                          ADDRESS                                                              PORTS     AGE
        demo-traffic-switching   myapp.example.org   example-lb-19tamgwi3atjf-1066321195.us-central-1.elb.amazonaws.com   80        1m

If it is provisioned you can check with curl, http to https redirect is created automatically by Skipper:

        curl -L -H"Host: myapp.example.org" example-lb-19tamgwi3atjf-1066321195.us-central-1.elb.amazonaws.com
        <body style='color: green; background-color: white;'><h1>Hello!</h1>

Check if Kops dns-controller created a DNS record:

        curl -L myapp.example.org
        <body style='color: green; background-color: white;'><h1>Hello!</h1>

You can now open your browser at
[https://myapp.example.org](https://myapp.example.org/) depending
on your `hostname` and reload it maybe 5 times to see switching from
white background to green background. If you modify the
`zalando.org/backend-weights` annotation you can control the chance
that you will hit the v1 or the v2 application. Use kubectl annotate to change this:

        kubectl annotate ingress demo-traffic-switching zalando.org/backend-weights='{"demo-app-v1": 20, "demo-app-v2": 80}'


### Cleanup

        for f in deploy/examples/sample*; do kubectl delete -f $f; done
        kubectl delete -f deploy/skipper.yaml
        kubectl delete -f deploy/ingress-controller.yaml
