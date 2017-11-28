# Test deployment

For this page we assume you have a running Kubernetes cluster with a
running kube-ingress-aws-controller and skipper deployed.
If not please read our [deployment readme](README.md)

## Base features

Deploy one sample application and change the hostname depending on
your route53 domain and ACM certificate:

```
kubectl create -f deploy/examples/sample-app-v1.yaml
kubectl create -f deploy/examples/sample-svc-v1.yaml
sed -i "s/<HOSTNAME>/demo-app.example.org/" deploy/examples/sample-ing-v1.yaml
kubectl create -f deploy/examples/sample-ing-v1.yaml
```

Check if your deployment was successful:

```
kubectl get pods,svc -l application=demo
```

To check if your Ingress created an ALB check the `ADDRESS` column:

```
kubectl get ing -l application=demo -o wide
NAME           HOSTS                          ADDRESS                                                              PORTS     AGE
demo-app-v1   myapp.example.org   example-lb-19tamgwi3atjf-1066321195.us-central-1.elb.amazonaws.com   80        1m
```

If it is provisioned you can check with curl, http to https redirect is created automatically by Skipper:

```
curl -L -H"Host: myapp.example.org" example-lb-19tamgwi3atjf-1066321195.us-central-1.elb.amazonaws.com
<body style='color: green; background-color: white;'><h1>Hello!</h1>
```

Check if Kops dns-controller created a DNS record:

```
curl -L myapp.example.org
<body style='color: green; background-color: white;'><h1>Hello!</h1>
```

## Advanced Features

We assume you have all components running that were applied in `Base features`.

Deploy a second ingress with a feature toggle and rate limit to protect you backend:

```
sed -i "s/<HOSTNAME>/demo-app.example.org/" deploy/examples/sample-ing-v2.yaml
kubectl create -f deploy/examples/sample-ing-v2.yaml
```

Deploy a second sample application:

```
kubectl create -f deploy/examples/sample-app-v2.yaml
kubectl create -f deploy/examples/sample-svc-v2.yaml
```

Now, you can test the feature toggle to access the new v2 application:

```
curl "https://myapp.example.org/?version=v2"
<body style='color: white; background-color: green;'><h1>Hello AWS!</h1>
```

If you run this more often, you can easily trigger the rate limit to stop proxying your call to the backend:

```
for i in {0..9}; do curl -v "https://myapp.example.org/?version=v2"; done
```

You should see output similar to:

```
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
```

Your endpoint is now protected.

Next we will show traffic switching.
Deploy an ingress with traffic switching 80% traffic goes to v1 and
20% to v2. Change the hostname depending on your route53 domain and
ACM certificate as before:

```
sed -i "s/<HOSTNAME>/demo-app.example.org/" deploy/examples/sample-ing-tf.yaml
kubectl create -f deploy/examples/sample-ing-traffic.yaml
```

Remove old ingress which will interfere with the new created one:

```
kubectl delete -f deploy/examples/sample-ing-v1.yaml
kubectl delete -f deploy/examples/sample-ing-v2.yaml
```

Check deployments and services (both should be 2)

```
kubectl get pods,svc -l application=demo
```

To check if your Ingress has an ALB check the `ADDRESS` column:

```
kubectl get ing -l application=demo -o wide
NAME           HOSTS                          ADDRESS                                                              PORTS     AGE
demo-traffic-switching   myapp.example.org   example-lb-19tamgwi3atjf-1066321195.us-central-1.elb.amazonaws.com   80        1m
```

If it is provisioned you can check with curl, http to https redirect is created automatically by Skipper:

```
curl -L -H"Host: myapp.example.org" example-lb-19tamgwi3atjf-1066321195.us-central-1.elb.amazonaws.com
<body style='color: green; background-color: white;'><h1>Hello!</h1>
```

Check if Kops dns-controller created a DNS record:

```
curl -L myapp.example.org
<body style='color: green; background-color: white;'><h1>Hello!</h1>
```

You can now open your browser at
[https://myapp.example.org](https://myapp.example.org/) depending
on your `hostname` and reload it maybe 5 times to see switching from
white background to green background. If you modify the
`zalando.org/backend-weights` annotation you can control the chance
that you will hit the v1 or the v2 application. Use kubectl annotate to change this:

```
kubectl annotate ingress demo-traffic-switching zalando.org/backend-weights='{"demo-app-v1": 20, "demo-app-v2": 80}'
```


# Cleanup

```
for f in deploy/examples/sample*; do kubectl delete -f $f; done
kubectl delete -f deploy/skipper.yaml
```

Wait a minute such that kube-ingress-aws-controller has time to delete
remaining ALBs and finally delete it from your cluster

```
kubectl delete -f deploy/ingress-controller.yaml
```
