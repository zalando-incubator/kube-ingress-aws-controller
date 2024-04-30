package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zalando-incubator/kube-ingress-aws-controller/kubernetes"
)

func TestDefaultLoadSettings(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd", "--target-access-mode", "HostPort"}

	err := loadSettings()

	require.NoError(t, err)
	require.Equal(t, "HostPort", targetAccessMode)
	require.Equal(t, false, versionFlag)
	require.Equal(t, false, debugFlag)
	require.Equal(t, false, quietFlag)
	require.Equal(t, "", apiServerBaseURL)
	require.Equal(t, 30*time.Second, pollingInterval)
	require.Equal(t, 5*time.Minute, creationTimeout)
	require.Equal(t, 30*time.Minute, certPollingInterval)
	require.Equal(t, false, disableSNISupport)
	require.Equal(t, false, disableInstrumentedHttpClient)
	require.Equal(t, false, stackTerminationProtection)
	require.Equal(t, map[string]string{}, additionalStackTags)
	require.Equal(t, 1*time.Hour, certTTL)
	require.Equal(t, "", certFilterTag)
	require.Equal(t, "/kube-system/healthz", healthCheckPath)
	require.Equal(t, uint(9999), healthCheckPort)
	require.Equal(t, uint(0), albHTTPTargetPort)
	require.Equal(t, uint(0), nlbHTTPTargetPort)
	require.Equal(t, false, targetHTTPS)
	require.Equal(t, "Not set", buildstamp)
	require.Equal(t, "Not set", githash)
	require.Equal(t, "Not set", version)
	require.Equal(t, 10*time.Second, healthCheckInterval)
	require.Equal(t, 5*time.Second, healthCheckTimeout)
	require.Equal(t, uint(5), albHealthyThresholdCount)
	require.Equal(t, uint(2), albUnhealthyThresholdCount)
	require.Equal(t, uint(3), nlbHealthyThresholdCount)
	require.Equal(t, uint(9999), targetPort)
	require.Equal(t, ":7979", metricsAddress)
	require.Equal(t, 1*time.Minute, idleConnectionTimeout)
	require.Equal(t, 5*time.Minute, deregistrationDelayTimeout)
	require.Equal(t, "", ingressClassFilters)
	require.Equal(t, "kube-ingress-aws-controller", controllerID)
	require.Equal(t, "", clusterID)
	require.Equal(t, "", vpcID)
	require.Equal(t, "", clusterLocalDomain)
	require.Equal(t, 24, maxCertsPerALB)
	require.Equal(t, "ELBSecurityPolicy-2016-08", sslPolicy)
	require.Equal(t, []string(nil), blacklistCertARNs)
	require.Equal(t, map[string]bool{}, blacklistCertArnMap)
	require.Equal(t, "ipv4", ipAddressType)
	require.Equal(t, "", albLogsS3Bucket)
	require.Equal(t, "", albLogsS3Prefix)
	require.Equal(t, "", wafWebAclId)
	require.Equal(t, false, httpRedirectToHTTPS)
	require.Equal(t, true, firstRun)
	require.Equal(t, "", cwAlarmConfigMap)
	require.Equal(t, (*kubernetes.ResourceLocation)(nil), cwAlarmConfigMapLocation)
	require.Equal(t, "application", loadBalancerType)
	require.Equal(t, "any_availability_zone", nlbZoneAffinity)
	require.Equal(t, false, nlbCrossZone)
	require.Equal(t, false, nlbHTTPEnabled)
	require.Equal(t, "networking.k8s.io/v1", ingressAPIVersion)
	require.Equal(t, []string{"*.cluster.local"}, internalDomains)
	require.Equal(t, "", targetCNINamespace)
	require.Equal(t, "", targetCNIPodLabelSelector)
	require.Equal(t, false, denyInternalDomains)
	require.Equal(t, "Unauthorized", denyInternalRespBody)
	require.Equal(t, "text/plain", denyInternalRespContentType)
	require.Equal(t, 401, denyInternalRespStatusCode)
	require.Equal(t, fmt.Sprintf("*%s", kubernetes.DefaultClusterLocalDomain), defaultInternalDomains)
}
