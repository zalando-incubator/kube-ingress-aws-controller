package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

type metrics struct {
	lastSyncTimestamp              prometheus.Gauge
	ingressesTotal                 prometheus.Gauge
	routegroupsTotal               prometheus.Gauge
	stacksTotal                    prometheus.Gauge
	ownedAutoscalingGroupsTotal    prometheus.Gauge
	targetedAutoscalingGroupsTotal prometheus.Gauge
	instancesTotal                 prometheus.Gauge
	standaloneInstancesTotal       prometheus.Gauge
	certificatesTotal              prometheus.Gauge
	cloudWatchAlarmsTotal          prometheus.Gauge
	changesTotal                   changeCounter
}

func newMetrics() *metrics {
	return &metrics{
		lastSyncTimestamp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "last_sync_timestamp_seconds",
				Help:      "Timestamp of the last successful controller reconciliation run",
			},
		),
		ingressesTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "ingresses_total",
				Help:      "Number of managed Kubernetes Ingresses",
			},
		),
		routegroupsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "routegroups_total",
				Help:      "Number of managed Route Groups",
			},
		),
		stacksTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "stacks_total",
				Help:      "Number of managed Cloud Formation stacks",
			},
		),
		ownedAutoscalingGroupsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "owned_autoscaling_groups_total",
				Help:      "Number of owned Autoscaling Groups",
			},
		),
		targetedAutoscalingGroupsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "targeted_autoscaling_groups_total",
				Help:      "Number of targeted Autoscaling Groups",
			},
		),
		instancesTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "instances_total",
				Help:      "Number of managed EC2 instances",
			},
		),
		standaloneInstancesTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "standalone_instances_total",
				Help:      "Number of managed EC2 instances not in the Autoscaling Group",
			},
		),
		certificatesTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "certificates_total",
				Help:      "Number of certificates",
			},
		),
		cloudWatchAlarmsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "cloud_watch_alarms_total",
				Help:      "Number of Cloud Watch Alarms",
			},
		),
		changesTotal: changeCounter{prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "kube_ingress_aws",
				Subsystem: "controller",
				Name:      "changes_total",
				Help:      "Number of Cloud Formation stack, Kubernetes Ingress and Route Group changes",
			},
			[]string{"resource_type", "operation"},
		)},
	}
}

type changeCounter struct {
	*prometheus.CounterVec
}

func (c changeCounter) created(resourceType string) {
	c.WithLabelValues(resourceType, "create").Inc()
}

func (c changeCounter) updated(resourceType string) {
	c.WithLabelValues(resourceType, "update").Inc()
}

func (c changeCounter) deleted(resourceType string) {
	c.WithLabelValues(resourceType, "delete").Inc()
}

func (metrics *metrics) serve(address string) {
	prometheus.MustRegister(metrics.lastSyncTimestamp)
	prometheus.MustRegister(metrics.ingressesTotal)
	prometheus.MustRegister(metrics.routegroupsTotal)
	prometheus.MustRegister(metrics.stacksTotal)
	prometheus.MustRegister(metrics.ownedAutoscalingGroupsTotal)
	prometheus.MustRegister(metrics.targetedAutoscalingGroupsTotal)
	prometheus.MustRegister(metrics.instancesTotal)
	prometheus.MustRegister(metrics.standaloneInstancesTotal)
	prometheus.MustRegister(metrics.certificatesTotal)
	prometheus.MustRegister(metrics.cloudWatchAlarmsTotal)
	prometheus.MustRegister(metrics.changesTotal)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(address, nil))
}
