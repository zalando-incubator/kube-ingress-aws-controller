// Package kubernetes provides some higher level Kubernetes abstractions to orchestrate Ingress resources.
//
// Operations
//
// The exported Adapter provides a limited set of operations that can be used to:
//  * List Ingress resources
//  * Update the Hostname attribute of Ingress load balancer objects
//
// Usage
//
// The Adapter can be created with the typical in-cluster configuration. This configuration depends on
// some specific Kubernetes environment variables and files, required to communicate with the API server:
//  * Environment variables KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT
//  * OAuth2 Bearer token contained in the file /var/run/secrets/kubernetes.io/serviceaccount/token
//  * The Root CA certificate contained in the file /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
//
// This is the preferred way and should be as simples as:
//
//  config, err := InClusterConfig()
//  if err != nil {
//      log.Fatal(err)
//  }
//  kubeAdapter, err := kubernetes.NewAdapter(config)
//  if err != nil {
//      log.Fatal(err)
//  }
//  ingresses, err := kubeAdapter.ListIngress() // for ex.
//
// For local development it is possible to create an Adapter using an insecure configuration.
//
// For example:
//
//  config := kubernetes.InsecureConfig("http://localhost:8001")
//  kubeAdapter, err := kubernetes.NewAdapter(config)
//  if err != nil {
//      log.Fatal(err)
//  }
//  ingresses, err := kubeAdapter.ListIngress() // for ex.
package kubernetes
