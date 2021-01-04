package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

const (
	defaultDisableSNISupport      = "false"
	defaultInstrumentedHttpClient = "false"
	defaultHTTPRedirectToHTTPS    = "false"
	defaultCertTTL                = "1h"
	customTagFilterEnvVarName     = "CUSTOM_FILTERS"
)

var (
	buildstamp                    = "Not set"
	githash                       = "Not set"
	version                       = "Not set"
	versionFlag                   bool
	apiServerBaseURL              string
	pollingInterval               time.Duration
	creationTimeout               time.Duration
	certPollingInterval           time.Duration
	healthCheckPath               string
	healthCheckPort               uint
	healthCheckInterval           time.Duration
	targetPort                    uint
	metricsAddress                string
	disableSNISupport             bool
	disableInstrumentedHttpClient bool
	certTTL                       time.Duration
	stackTerminationProtection    bool
	additionalStackTags           = make(map[string]string)
	idleConnectionTimeout         time.Duration
	deregistrationDelayTimeout    time.Duration
	ingressClassFilters           string
	controllerID                  string
	clusterID                     string
	vpcID                         string
	clusterLocalDomain            string
	maxCertsPerALB                int
	sslPolicy                     string
	blacklistCertARNs             []string
	blacklistCertArnMap           map[string]bool
	ipAddressType                 string
	albLogsS3Bucket               string
	albLogsS3Prefix               string
	wafWebAclId                   string
	httpRedirectToHTTPS           bool
	debugFlag                     bool
	quietFlag                     bool
	firstRun                      bool = true
	cwAlarmConfigMap              string
	cwAlarmConfigMapLocation      *kubernetes.ResourceLocation
	loadBalancerType              string
	nlbCrossZone                  bool
	nlbHTTPEnabled                bool
	ingressAPIVersion             string
)

func loadSettings() error {
	kingpin.Flag("version", "Print version and exit").Default("false").BoolVar(&versionFlag)
	kingpin.Flag("debug", "Enables debug logging level").Default("false").BoolVar(&debugFlag)
	kingpin.Flag("quiet", "Enables quiet logging").Default("false").BoolVar(&quietFlag)
	kingpin.Flag("api-server-base-url", "sets the kubernetes api server base url. If empty will try to use the configuration from the running cluster, else it will use InsecureConfig, that does not use encryption or authentication (use case to develop with kubectl proxy).").
		Envar("API_SERVER_BASE_URL").StringVar(&apiServerBaseURL)
	kingpin.Flag("polling-interval", "sets the polling interval for ingress resources. The flag accepts a value acceptable to time.ParseDuration").
		Envar("POLLING_INTERVAL").Default("30s").DurationVar(&pollingInterval)
	kingpin.Flag("creation-timeout", "sets the stack creation timeout. The flag accepts a value acceptable to time.ParseDuration. Should be >= 1min").
		Envar("CREATION_TIMEOUT").Default(aws.DefaultCreationTimeout.String()).DurationVar(&creationTimeout)
	kingpin.Flag("cert-polling-interval", "sets the polling interval for the certificates cache refresh. The flag accepts a value acceptable to time.ParseDuration").
		Envar("CERT_POLLING_INTERVAL").Default(aws.DefaultCertificateUpdateInterval.String()).DurationVar(&certPollingInterval)
	kingpin.Flag("disable-sni-support", "disables SNI support limiting the number of certificates per ALB to 1.").
		Default(defaultDisableSNISupport).BoolVar(&disableSNISupport)
	kingpin.Flag("disable-instrumented-http-client", "disables instrumented http client.").
		Default(defaultInstrumentedHttpClient).BoolVar(&disableInstrumentedHttpClient)
	kingpin.Flag("stack-termination-protection", "enables stack termination protection for the stacks managed by the controller.").
		Default("false").BoolVar(&stackTerminationProtection)
	kingpin.Flag("additional-stack-tags", "set additional custom tags on the Cloudformation Stacks managed by the controller.").
		StringMapVar(&additionalStackTags)
	kingpin.Flag("cert-ttl-timeout", "sets the timeout of how long a certificate is kept on an old ALB to be decommissioned.").
		Default(defaultCertTTL).DurationVar(&certTTL)
	kingpin.Flag("health-check-path", "sets the health check path for the created target groups").
		Default(aws.DefaultHealthCheckPath).StringVar(&healthCheckPath)
	kingpin.Flag("health-check-port", "sets the health check port for the created target groups").
		Default(strconv.FormatUint(aws.DefaultHealthCheckPort, 10)).UintVar(&healthCheckPort)
	kingpin.Flag("target-port", "sets the target port for the created target groups").
		Default(strconv.FormatUint(aws.DefaultTargetPort, 10)).UintVar(&targetPort)
	kingpin.Flag("health-check-interval", "sets the health check interval for the created target groups. The flag accepts a value acceptable to time.ParseDuration").
		Default(aws.DefaultHealthCheckInterval.String()).DurationVar(&healthCheckInterval)
	kingpin.Flag("idle-connection-timeout", "sets the idle connection timeout of all ALBs. The flag accepts a value acceptable to time.ParseDuration and are between 1s and 4000s.").
		Default(aws.DefaultIdleConnectionTimeout.String()).DurationVar(&idleConnectionTimeout)
	kingpin.Flag("deregistration-delay-timeout", "sets the deregistration delay timeout of all target groups.  The flag accepts a value acceptable to time.ParseDuration that is between 1s and 3600s.").
		Default(aws.DefaultDeregistrationTimeout.String()).DurationVar(&deregistrationDelayTimeout)
	kingpin.Flag("metrics-address", "defines where to serve metrics").Default(":7979").StringVar(&metricsAddress)
	kingpin.Flag("ingress-class-filter", "optional comma-seperated list of kubernetes.io/ingress.class annotation values to filter behaviour on.").
		StringVar(&ingressClassFilters)
	kingpin.Flag("controller-id", "controller ID used to differentiate resources from multiple aws ingress controller instances").
		Default(aws.DefaultControllerID).StringVar(&controllerID)
	kingpin.Flag("cluster-id", "ID of the Kubernetes cluster used to lookup cluster related resources tagged with `kubernetes.io/cluster/<cluster-id>` tags. Auto discovered from the EC2 instance where the controller is running if not specified.").
		StringVar(&clusterID)
	kingpin.Flag("vpc-id", "VPC ID for where the cluster is running. Used to lookup relevant subnets. Auto discovered from the EC2 instance where the controller is running if not specified.").
		StringVar(&vpcID)
	kingpin.Flag("cluster-local-domain", "Cluster local domain is used to detect hostnames, that won't trigger a creation of an AWS load balancer, empty string will not change the default behavior. In Kubernetes you might want to pass cluster.local").
		Default("").StringVar(&clusterLocalDomain)
	kingpin.Flag("max-certs-alb", fmt.Sprintf("sets the maximum number of certificates to be attached to an ALB. Cannot be higher than %d", aws.DefaultMaxCertsPerALB)).
		Default(strconv.Itoa(aws.DefaultMaxCertsPerALB)).IntVar(&maxCertsPerALB) // TODO: max
	kingpin.Flag("ssl-policy", "Security policy that will define the protocols/ciphers accepts by the SSL listener").
		Default(aws.DefaultSslPolicy).EnumVar(&sslPolicy, aws.SSLPoliciesList...)
	kingpin.Flag("blacklist-certificate-arns", "Certificate ARNs to not consider by the controller.").StringsVar(&blacklistCertARNs)
	kingpin.Flag("ip-addr-type", "IP Address type to use.").
		Default(aws.DefaultIpAddressType).EnumVar(&ipAddressType, aws.IPAddressTypeIPV4, aws.IPAddressTypeDualstack)
	kingpin.Flag("logs-s3-bucket", "S3 bucket to be used for ALB logging").
		Default(aws.DefaultAlbS3LogsBucket).StringVar(&albLogsS3Bucket)
	kingpin.Flag("logs-s3-prefix", "Prefix within S3 bucket to be used for ALB logging").
		Default(aws.DefaultAlbS3LogsPrefix).StringVar(&albLogsS3Prefix)
	kingpin.Flag("aws-waf-web-acl-id", "WAF web acl id to be associated with the ALB. For WAF v2 it is possible to specify the WebACL ARN arn:aws:wafv2:<region>:<account>:regional/webacl/<name>/<id>").
		Default("").StringVar(&wafWebAclId)
	kingpin.Flag("cloudwatch-alarms-config-map", "ConfigMap location of the form 'namespace/config-map-name' where to read CloudWatch Alarm configuration from. Ignored if empty.").
		StringVar(&cwAlarmConfigMap)
	kingpin.Flag("redirect-http-to-https", "Configure HTTP listener to redirect to HTTPS").
		Default(defaultHTTPRedirectToHTTPS).BoolVar(&httpRedirectToHTTPS)
	kingpin.Flag("load-balancer-type", "Sets default Load Balancer type (application or network).").
		Default(aws.LoadBalancerTypeApplication).EnumVar(&loadBalancerType, aws.LoadBalancerTypeApplication, aws.LoadBalancerTypeNetwork)
	kingpin.Flag("nlb-cross-zone", "Specify whether Network Load Balancers should balance cross availablity zones. This setting only apply to 'network' Load Balancers.").
		Default("false").BoolVar(&nlbCrossZone)
	kingpin.Flag("nlb-http-enabled", "Enable HTTP (port 80) for Network Load Balancers. By default this is disabled as NLB can't provide HTTP -> HTTPS redirect.").
		Default("false").BoolVar(&nlbHTTPEnabled)
	kingpin.Flag("ingress-api-version", "APIversion used for listing/updating ingresses.").
		Default(kubernetes.IngressAPIVersionNetworking).EnumVar(&ingressAPIVersion, kubernetes.IngressAPIVersionNetworking, kubernetes.IngressAPIVersionExtensions)
	kingpin.Parse()

	blacklistCertArnMap = make(map[string]bool)
	for _, s := range blacklistCertARNs {
		blacklistCertArnMap[s] = true
	}

	if creationTimeout < 1*time.Minute {
		return fmt.Errorf("invalid creation timeout %d. please specify a value > 1min", creationTimeout)
	}

	if healthCheckPort == 0 || healthCheckPort > 65535 {
		return fmt.Errorf("invalid health check port: %d. please use a valid TCP port", healthCheckPort)
	}

	if targetPort == 0 || targetPort > 65535 {
		return fmt.Errorf("invalid target port: %d. please use a valid TCP port", targetPort)
	}

	if maxCertsPerALB > aws.DefaultMaxCertsPerALB {
		return fmt.Errorf("invalid max number of certificates per ALB: %d. AWS does not allow more than %d", maxCertsPerALB, aws.DefaultMaxCertsPerALB)
	}

	if cwAlarmConfigMap != "" {
		loc, err := kubernetes.ParseResourceLocation(cwAlarmConfigMap)
		if err != nil {
			return fmt.Errorf("failed to parse cloudwatch alarm config map location: %v", err)
		}

		cwAlarmConfigMapLocation = loc
	}

	if quietFlag && debugFlag {
		log.Warn("--quiet and --debug flags are both set. Debug will be used as logging level.")
	}

	if quietFlag {
		log.SetLevel(log.WarnLevel)
	}

	if debugFlag {
		log.SetLevel(log.DebugLevel)
	}

	log.SetOutput(os.Stdout)

	return nil
}

func main() {
	log.Infof("starting %s %s", os.Args[0], version)
	var (
		awsAdapter  *aws.Adapter
		kubeAdapter *kubernetes.Adapter
		kubeConfig  *kubernetes.Config
		err         error
	)
	if err = loadSettings(); err != nil {
		log.Fatal(err)
	}

	if versionFlag {
		log.Infof(`%s
===========================
  Version: %s
  Buildtime: %s
  GitHash: %s
`, path.Base(os.Args[0]), version, buildstamp, githash)
		os.Exit(0)
	}

	log.Debug("aws.NewAdapter")
	awsAdapter, err = aws.NewAdapter(clusterID, controllerID, vpcID, debugFlag, disableInstrumentedHttpClient)
	if err != nil {
		log.Fatal(err)
	}

	customFilter, ok := os.LookupEnv(customTagFilterEnvVarName)
	if !ok {
		customFilter = ""
	}

	awsAdapter = awsAdapter.
		WithHealthCheckPath(healthCheckPath).
		WithHealthCheckPort(healthCheckPort).
		WithHealthCheckInterval(healthCheckInterval).
		WithTargetPort(targetPort).
		WithCreationTimeout(creationTimeout).
		WithStackTerminationProtection(stackTerminationProtection).
		WithIdleConnectionTimeout(idleConnectionTimeout).
		WithDeregistrationDelayTimeout(deregistrationDelayTimeout).
		WithControllerID(controllerID).
		WithSslPolicy(sslPolicy).
		WithIpAddressType(ipAddressType).
		WithAlbLogsS3Bucket(albLogsS3Bucket).
		WithAlbLogsS3Prefix(albLogsS3Prefix).
		WithHTTPRedirectToHTTPS(httpRedirectToHTTPS).
		WithNLBCrossZone(nlbCrossZone).
		WithNLBHTTPEnabled(nlbHTTPEnabled).
		WithCustomFilter(customFilter).
		WithStackTags(additionalStackTags)

	log.Debug("certs.NewCachingProvider")
	certificatesProvider, err := certs.NewCachingProvider(
		certPollingInterval,
		blacklistCertArnMap,
		awsAdapter.NewACMCertificateProvider(),
		awsAdapter.NewIAMCertificateProvider(),
	)
	if err != nil {
		log.Fatal(err)
	}

	if apiServerBaseURL == "" {
		log.Debug("kubernetes.InClusterConfig")
		kubeConfig, err = kubernetes.InClusterConfig()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Debug("kubernetes.InsecureConfig")
		kubeConfig = kubernetes.InsecureConfig(apiServerBaseURL)
	}

	ingressClassFiltersList := []string{}
	if ingressClassFilters != "" {
		ingressClassFiltersList = strings.Split(ingressClassFilters, ",")
	}

	log.Debug("kubernetes.NewAdapter")
	kubeAdapter, err = kubernetes.NewAdapter(kubeConfig, ingressAPIVersion, ingressClassFiltersList, awsAdapter.SecurityGroupID(), sslPolicy, loadBalancerType, clusterLocalDomain, disableInstrumentedHttpClient)
	if err != nil {
		log.Fatal(err)
	}

	certificatesPerALB := maxCertsPerALB
	if disableSNISupport {
		certificatesPerALB = 1
	}

	log.Info("controller manifest:")
	log.Infof("Kubernetes API server: %s", apiServerBaseURL)
	log.Infof("Cluster ID: %s", awsAdapter.ClusterID())
	log.Infof("VPC ID: %s", awsAdapter.VpcID())
	log.Infof("Instance ID: %s", awsAdapter.InstanceID())
	log.Infof("Security group ID: %s", awsAdapter.SecurityGroupID())
	log.Infof("Internal subnet IDs: %s", awsAdapter.FindLBSubnets(elbv2.LoadBalancerSchemeEnumInternal))
	log.Infof("Public subnet IDs: %s", awsAdapter.FindLBSubnets(elbv2.LoadBalancerSchemeEnumInternetFacing))
	log.Infof("EC2 filters: %s", awsAdapter.FiltersString())
	log.Infof("Certificates per ALB: %d (SNI: %t)", certificatesPerALB, certificatesPerALB > 1)
	log.Infof("Blacklisted Certificate ARNs (%d): %s", len(blacklistCertARNs), strings.Join(blacklistCertARNs, ","))
	log.Infof("Ingress class filters: %s", kubeAdapter.IngressFiltersString())
	log.Infof("ALB Logging S3 Bucket: %s", awsAdapter.S3Bucket())
	log.Infof("ALB Logging S3 Prefix: %s", awsAdapter.S3Prefix())
	log.Infof("CloudWatch Alarm ConfigMap: %s", cwAlarmConfigMapLocation)
	log.Infof("Default LoadBalancer type: %s", loadBalancerType)

	ctx, cancel := context.WithCancel(context.Background())
	go handleTerminationSignals(cancel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go serveMetrics(metricsAddress)
	startPolling(
		ctx,
		certificatesProvider,
		certificatesPerALB,
		certTTL,
		awsAdapter,
		kubeAdapter,
		pollingInterval,
		wafWebAclId,
	)

	log.Infof("Terminating %s", os.Args[0])
}

func handleTerminationSignals(cancelFunc func(), signals ...os.Signal) {
	sigsc := make(chan os.Signal, 1)
	signal.Notify(sigsc, signals...)
	<-sigsc
	cancelFunc()
}

func serveMetrics(address string) {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(address, nil))
}
