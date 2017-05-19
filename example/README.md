# Examples

## Requirement

* You have a running Kubernetes Cluster on AWS
* You have successfully
  [deployed](https://github.com/zalando-incubator/kube-ingress-aws-controller/tree/master/deploy)
  the example setup with [skipper](https://github.com/zalando/skipper)
  and added all
  [permissions](https://github.com/zalando-incubator/kube-ingress-aws-controller/tree/master/deploy/requirements.md)
  to kube-ingress-aws-incontroller.
* You have a route53 Hosted Zone in your AWS account.
* You have provisioned a valid ACM or IAM certificate that is valid for **my.YOUR_HOSTED_ZONE**
* Optional to manage route53 DNS records automatically you can install
  [external-dns](https://github.com/kubernetes-incubator/external-dns/)

## Run the simple Example

In directory
[simple](simple)
you find a `deployment`, a `service` and an `ingress` spec.
You should change in the `ingress` resource
**my-nginx-new.YOUR_HOSTED_ZONE** to match your route53 Hosted Zone.
After that you can deploy the simple example:

    cd simple
    for f in *.yaml; do kubectl create -f $f; done

To check if everything worked you should check the **status** object of
your ingress object, which could look similar to this one:

    status:
      loadBalancer:
        ingress:
        - hostname: aws-1708-lb-qc2a3dlwwv31-772637261.eu-central-1.elb.amazonaws.com

If the status field is changed it means the ALB is provisioned, if not
you should have a look at the Cloudformation tab, this controller
should create this stack and if it does not work try to recreate every
minute. Watch for errors there.

If you have
[external-dns](https://github.com/kubernetes-incubator/external-dns/)
running in your cluster you should also have now the specified
**my-nginx-new.YOUR_HOSTED_ZONE** DNS name provisioned to point to the
created ALB. If not you have to manually create a DNS record to point
to the ALB hostname from the ingress status object.

Now you can query https://my.YOUR_HOSTED_ZONE/ with your browser and
it shows the default nginx welcome page.
For example:

    % curl https://my.YOUR_HOSTED_ZONE/
    <!DOCTYPE html>
    <html>
    <head>
    <title>Welcome to nginx!</title>
    <style>
        body {
            width: 35em;
            margin: 0 auto;
            font-family: Tahoma, Verdana, Arial, sans-serif;
        }
    </style>
    </head>
    <body>
    <h1>Welcome to nginx!</h1>
    <p>If you see this page, the nginx web server is successfully installed and
    working. Further configuration is required.</p>

    <p>For online documentation and support please refer to
    <a href="http://nginx.org/">nginx.org</a>.<br/>
    Commercial support is available at
    <a href="http://nginx.com/">nginx.com</a>.</p>

    <p><em>Thank you for using nginx.</em></p>
    </body>
    </html>

You also get a redirect from http to https for free if you use
[skipper](https://github.com/zalando/skipper) as ingress implementation.

    % curl -I my-nginx.YOUR_HOSTED_ZONE
    HTTP/1.1 301 Moved Permanently
    Date: Fri, 19 May 2017 15:27:29 GMT
    Content-Type: text/plain; charset=utf-8
    Connection: keep-alive
    Location: https://my-nginx.YOUR_HOSTED_ZONE/
    Server: Skipper
