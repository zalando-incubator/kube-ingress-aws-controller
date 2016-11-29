package k8s

/*
{"type":"ADDED","object":{"kind":"Ingress","apiVersion":"extensions/v1beta1","metadata":{"name":"echoheaders-map","namespace":"default","selfLink":"/apis/extensions/v1beta1/namespaces/default/ingresses/echoheaders-map","uid":"9f03c70f-b643-11e6-8d9b-e65b27ab04c7","resourceVersion":"2364","generation":1,"creationTimestamp":"2016-11-29T14:53:42Z"},"spec":{"rules":[{"host":"x.echoheaders.dev","http":{"paths":[{"path":"/foo","backend":{"serviceName":"echoheaders-x","servicePort":80}}]}},{"host":"y.echoheaders.dev","http":{"paths":[{"path":"/foo","backend":{"serviceName":"echoheaders-y","servicePort":80}},{"path":"/bar","backend":{"serviceName":"echoheaders-x","servicePort":80}}]}}]},"status":{"loadBalancer":{}}}}
 */

func Watch() {

}