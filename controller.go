package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"flag"
	"time"

	"io/ioutil"

	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
)

func loadSettings() error {
	flag.Usage = usage
	flag.BoolVar(&versionFlag, "version", false, "Print version and exit")
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

	flag.Parse()

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
	log.Printf("starting %s", os.Args[0])
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
		fmt.Printf(`%s
===========================
  Version: %s
  Buildtime: %s
  GitHash: %s
`, path.Base(os.Args[0]), version, buildstamp, githash)
		os.Exit(0)
	}

	awsAdapter, err = aws.NewAdapter()
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
		WithControllerID(controllerID)

	certificatesProvider, err := certs.NewCachingProvider(
		certPollingInterval,
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

	if ingressClassFilters == "" {
		kubeAdapter, err = kubernetes.NewAdapter(kubeConfig, []string{})
	} else {
		kubeAdapter, err = kubernetes.NewAdapter(kubeConfig, strings.Split(ingressClassFilters, ","))
	}

	if err != nil {
		log.Fatal(err)
	}

	certificatesPerALB := maxCertsPerALB
	if disableSNISupport {
		certificatesPerALB = 1
	}

	log.Println("controller manifest:")
	log.Printf("\tKubernetes API server: %s", apiServerBaseURL)
	log.Printf("\tCluster ID: %s", awsAdapter.ClusterID())
	log.Printf("\tVPC ID: %s", awsAdapter.VpcID())
	log.Printf("\tInstance ID: %s", awsAdapter.InstanceID())
	log.Printf("\tSecurity group ID: %s", awsAdapter.SecurityGroupID())
	log.Printf("\tInternal subnet IDs: %s", awsAdapter.FindLBSubnets(elbv2.LoadBalancerSchemeEnumInternal))
	log.Printf("\tPublic subnet IDs: %s", awsAdapter.FindLBSubnets(elbv2.LoadBalancerSchemeEnumInternetFacing))
	log.Printf("\tEC2 filters: %s", awsAdapter.FiltersString())
	log.Printf("\tCertificates per ALB: %d (SNI: %t)", certificatesPerALB, certificatesPerALB > 1)
	log.Printf("\tIngress class filters: %s", kubeAdapter.IngressFiltersString())

	go serveMetrics(metricsAddress)
	quitCH := make(chan struct{})
	go startPolling(quitCH, certificatesProvider, certificatesPerALB, certTTL, awsAdapter, kubeAdapter, pollingInterval)
	<-quitCH

	log.Printf("terminating %s", os.Args[0])
}

func serveMetrics(address string) {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(address, nil))
}
