package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"flag"
	"time"

	"io/ioutil"

	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/zalando-incubator/kube-ingress-aws-controller/aws"
	"github.com/zalando-incubator/kube-ingress-aws-controller/certs"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

const (
	defaultDisableSNISupport = false
	defaultCertTTL           = 30 * time.Minute
)

var (
	buildstamp                 = "Not set"
	githash                    = "Not set"
	version                    = "Not set"
	versionFlag                bool
	apiServerBaseURL           string
	pollingInterval            time.Duration
	cfCustomTemplate           string
	creationTimeout            time.Duration
	certPollingInterval        time.Duration
	healthCheckPath            string
	healthCheckPort            uint
	healthCheckInterval        time.Duration
	targetPort                 uint
	metricsAddress             string
	disableSNISupport          bool
	certTTL                    time.Duration
	stackTerminationProtection bool
	idleConnectionTimeout      time.Duration
	ingressClassFilters        string
	controllerID               string
	maxCertsPerALB             int
	sslPolicy                  string
	blacklistCertARN           string
	blacklistCertArnMap        map[string]bool
	ipAddressType              string
	albLogsS3Bucket            string
	albLogsS3Prefix            string
	wafWebAclId                string
	debugFlag                  bool
	quietFlag                  bool
	firstRun                   bool = true
)

func loadSettings() error {
	flag.Usage = usage
	flag.BoolVar(&versionFlag, "version", false, "Print version and exit")
	flag.BoolVar(&debugFlag, "debug", false, "Enables debug logging level")
	flag.BoolVar(&quietFlag, "quiet", false, "Enables quiet logging")
	flag.StringVar(&apiServerBaseURL, "api-server-base-url", "", "sets the kubernetes api "+
		"server base url. If empty will try to use the configuration from the running cluster, else it will use InsecureConfig, that does not use encryption or authentication (use case to develop with kubectl proxy).")
	flag.DurationVar(&pollingInterval, "polling-interval", 30*time.Second, "sets the polling interval for "+
		"ingress resources. The flag accepts a value acceptable to time.ParseDuration")
	flag.StringVar(&cfCustomTemplate, "cf-custom-template", "",
		"filename for a custom cloud formation template to use instead of the built in")
	flag.DurationVar(&creationTimeout, "creation-timeout", aws.DefaultCreationTimeout,
		"sets the stack creation timeout. The flag accepts a value acceptable to time.ParseDuration. "+
			"Should be >= 1min")
	flag.DurationVar(&certPollingInterval, "cert-polling-interval", aws.DefaultCertificateUpdateInterval,
		"sets the polling interval for the certificates cache refresh. The flag accepts a value "+
			"acceptable to time.ParseDuration")
	flag.BoolVar(&disableSNISupport, "disable-sni-support", defaultDisableSNISupport, "disables SNI support limiting the number of certificates per ALB to 1.")
	flag.BoolVar(&stackTerminationProtection, "stack-termination-protection", false, "enables stack termination protection for the stacks managed by the controller.")
	flag.DurationVar(&certTTL, "cert-ttl-timeout", defaultCertTTL,
		"sets the timeout of how long a certificate is kept on an old ALB to be decommissioned.")
	flag.StringVar(&healthCheckPath, "health-check-path", aws.DefaultHealthCheckPath,
		"sets the health check path for the created target groups")
	flag.UintVar(&healthCheckPort, "health-check-port", aws.DefaultHealthCheckPort,
		"sets the health check port for the created target groups")
	flag.UintVar(&targetPort, "target-port", 9999,
		"sets the target port for the created target groups")
	flag.DurationVar(&healthCheckInterval, "health-check-interval", aws.DefaultHealthCheckInterval,
		"sets the health check interval for the created target groups. The flag accepts a value "+
			"acceptable to time.ParseDuration")
	flag.DurationVar(&idleConnectionTimeout, "idle-connection-timeout", aws.DefaultIdleConnectionTimeout,
		"sets the idle connection timeout of all ALBs. The flag accepts a value acceptable to time.ParseDuration and are between 1s and 4000s.")
	flag.StringVar(&metricsAddress, "metrics-address", ":7979", "defines where to serve metrics")
	flag.StringVar(&ingressClassFilters, "ingress-class-filter", "", "optional comma-seperated list of kubernetes.io/ingress.class annotation values to filter behaviour on. ")
	flag.StringVar(&controllerID, "controller-id", aws.DefaultControllerID, "controller ID used to differentiate resources from multiple aws ingress controller instances")
	flag.IntVar(&maxCertsPerALB, "max-certs-alb", aws.DefaultMaxCertsPerALB,
		fmt.Sprintf("sets the maximum number of certificates to be attached to an ALB. Cannot be higher than %d", aws.DefaultMaxCertsPerALB))
	flag.StringVar(&sslPolicy, "ssl-policy", aws.DefaultSslPolicy, "Security policy that will define the protocols/ciphers accepts by the SSL listener")
	flag.StringVar(&blacklistCertARN, "blacklist-certificate-arns", "", "Certificate ARNs to not consider by the controller: arn1,arn2,..")
	flag.StringVar(&ipAddressType, "ip-addr-type", aws.DefaultIpAddressType, "IP Address type to use, one of 'ipv4' or 'dualstack'")
	flag.StringVar(&albLogsS3Bucket, "logs-s3-bucket", aws.DefaultAlbS3LogsBucket, "S3 bucket to be used for ALB logging")
	flag.StringVar(&albLogsS3Prefix, "logs-s3-prefix", aws.DefaultAlbS3LogsPrefix, "Prefix within S3 bucket to be used for ALB logging")
	flag.StringVar(&wafWebAclId, "aws-waf-web-acl-id", aws.DefaultWafWebAclId, "Waf web acl id to be associated with the ALB")

	flag.Parse()

	blacklistCertArnMap = make(map[string]bool)
	for _, s := range strings.Split(blacklistCertARN, ",") {
		blacklistCertArnMap[s] = true
	}

	if tmp, defined := os.LookupEnv("API_SERVER_BASE_URL"); defined {
		apiServerBaseURL = tmp
	}

	if err := loadDurationFromEnv("POLLING_INTERVAL", &pollingInterval); err != nil {
		return err
	}

	if err := loadDurationFromEnv("CREATION_TIMEOUT", &creationTimeout); err != nil {
		return err
	}
	if creationTimeout < 1*time.Minute {
		return fmt.Errorf("invalid creation timeout %d. please specify a value > 1min", creationTimeout)
	}

	if err := loadDurationFromEnv("CERT_POLLING_INTERVAL", &certPollingInterval); err != nil {
		return err
	}

	if healthCheckPort == 0 || healthCheckPort > 65535 {
		return fmt.Errorf("invalid health check port: %d. please use a valid TCP port", healthCheckPort)
	}

	if targetPort == 0 || targetPort > 65535 {
		return fmt.Errorf("invalid target port: %d. please use a valid TCP port", targetPort)
	}

	if cfCustomTemplate != "" {
		buf, err := ioutil.ReadFile(cfCustomTemplate)
		if err != nil {
			return err
		}
		cfCustomTemplate = string(buf)
	}

	if maxCertsPerALB > aws.DefaultMaxCertsPerALB {
		return fmt.Errorf("invalid max number of certificates per ALB: %d. AWS does not allow more than %d", maxCertsPerALB, aws.DefaultMaxCertsPerALB)
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

func loadDurationFromEnv(varName string, dest *time.Duration) error {
	if tmp, defined := os.LookupEnv(varName); defined {
		interval, err := time.ParseDuration(tmp)
		if err != nil || interval <= 0 {
			return err
		}
		*dest = interval
	}
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [options]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "where options can be:")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.Infof("starting %s", os.Args[0])
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

	awsAdapter, err = aws.NewAdapter(controllerID)
	if err != nil {
		log.Fatal(err)
	}
	awsAdapter = awsAdapter.
		WithHealthCheckPath(healthCheckPath).
		WithHealthCheckPort(healthCheckPort).
		WithHealthCheckInterval(healthCheckInterval).
		WithTargetPort(targetPort).
		WithCreationTimeout(creationTimeout).
		WithCustomTemplate(cfCustomTemplate).
		WithStackTerminationProtection(stackTerminationProtection).
		WithIdleConnectionTimeout(idleConnectionTimeout).
		WithControllerID(controllerID).
		WithSslPolicy(sslPolicy).
		WithIpAddressType(ipAddressType).
		WithAlbLogsS3Bucket(albLogsS3Bucket).
		WithAlbLogsS3Prefix(albLogsS3Prefix).
		WithWafWebAclId(wafWebAclId)

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
		kubeConfig, err = kubernetes.InClusterConfig()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		kubeConfig = kubernetes.InsecureConfig(apiServerBaseURL)
	}

	ingressClassFiltersList := []string{}
	if ingressClassFilters != "" {
		ingressClassFiltersList = strings.Split(ingressClassFilters, ",")
	}

	kubeAdapter, err = kubernetes.NewAdapter(kubeConfig, ingressClassFiltersList, awsAdapter.SecurityGroupID(), sslPolicy)
	if err != nil {
		log.Fatal(err)
	}

	certificatesPerALB := maxCertsPerALB
	if disableSNISupport {
		certificatesPerALB = 1
	}

	log.Info("controller manifest:")
	log.Infof("\tKubernetes API server: %s", apiServerBaseURL)
	log.Infof("\tCluster ID: %s", awsAdapter.ClusterID())
	log.Infof("\tVPC ID: %s", awsAdapter.VpcID())
	log.Infof("\tInstance ID: %s", awsAdapter.InstanceID())
	log.Infof("\tSecurity group ID: %s", awsAdapter.SecurityGroupID())
	log.Infof("\tInternal subnet IDs: %s", awsAdapter.FindLBSubnets(elbv2.LoadBalancerSchemeEnumInternal))
	log.Infof("\tPublic subnet IDs: %s", awsAdapter.FindLBSubnets(elbv2.LoadBalancerSchemeEnumInternetFacing))
	log.Infof("\tEC2 filters: %s", awsAdapter.FiltersString())
	log.Infof("\tCertificates per ALB: %d (SNI: %t)", certificatesPerALB, certificatesPerALB > 1)
	log.Infof("\tBlacklisted Certificate ARNs (%d): %s", len(blacklistCertArnMap), blacklistCertARN)
	log.Infof("\tIngress class filters: %s", kubeAdapter.IngressFiltersString())
	log.Infof("\tALB Logging S3 Bucket: %s", awsAdapter.S3Bucket())
	log.Infof("\tALB Logging S3 Prefix: %s", awsAdapter.S3Prefix())

	ctx, cancel := context.WithCancel(context.Background())
	go handleTerminationSignals(cancel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go serveMetrics(metricsAddress)
	startPolling(ctx, certificatesProvider, certificatesPerALB, certTTL, awsAdapter, kubeAdapter, pollingInterval)

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
