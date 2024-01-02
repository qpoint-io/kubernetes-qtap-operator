package v1

import (
	"fmt"
	"math"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const INIT_IMAGE = "us-docker.pkg.dev/qpoint-edge/public/kubernetes-qtap-init"
const QTAP_IMAGE = "us-docker.pkg.dev/qpoint-edge/public/qtap"

var (
	ROOT_USER       int64 = 0 // The root user
	ROOT_GROUP      int64 = 0 // The root group
	RUN_AS_NON_ROOT       = false
)

func MutateEgress(pod *corev1.Pod, config *Config) error {
	// fetch the init image tag
	tag := config.GetAnnotation("egress-init-tag")

	// create an init container
	initContainer := corev1.Container{
		Name:  "qtap-init",
		Image: fmt.Sprintf("%s:%s", INIT_IMAGE, tag),
		Env:   []corev1.EnvVar{},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
			// The init container needs to run as root as it modifies the network
			// for the pod
			RunAsUser:    &ROOT_USER,
			RunAsGroup:   &ROOT_GROUP,
			RunAsNonRoot: &RUN_AS_NON_ROOT, // Allow running as root
		},
	}

	// TO_ADDR
	if toAddr := config.GetAnnotation("egress-to-addr"); toAddr != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "TO_ADDR",
			Value: toAddr,
		})
	}

	// TO_DOMAIN
	if toDomain := config.GetAnnotation("egress-to-domain"); toDomain != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "TO_DOMAIN",
			Value: toDomain,
		})
	}

	// PORT_MAPPING
	if portMapping := config.GetAnnotation("egress-port-mapping"); portMapping != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "PORT_MAPPING",
			Value: portMapping,
		})
	}

	// ACCEPT_UIDS
	if acceptUids := config.GetAnnotation("egress-accept-uids"); acceptUids != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "ACCEPT_UIDS",
			Value: acceptUids,
		})
	}

	// ACCEPT_GIDS
	if acceptGids := config.GetAnnotation("egress-accept-gids"); acceptGids != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "ACCEPT_GIDS",
			Value: acceptGids,
		})
	}

	// ensure init containers has been initialized
	if pod.Spec.InitContainers == nil {
		pod.Spec.InitContainers = make([]corev1.Container, 0)
	}

	// append to the list
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, initContainer)

	// gtg
	return nil
}

func MutateInjection(pod *corev1.Pod, config *Config) error {
	// in order to start qtap a token is needed. This token can be found at a defined secret name token
	secret := &corev1.Secret{}
	if err := config.Client.Get(config.Ctx, client.ObjectKey{Name: "token", Namespace: config.OperatorNamespace}, secret); err != nil {
		return fmt.Errorf("fetching secret '%s' at namespace '%s' from the api: %w", "token", config.OperatorNamespace, err)
	}

	tokenBytes, exists := secret.Data["token"]
	if !exists {
		return fmt.Errorf("token not found in secret '%s'", "token")
	}

	// convert the []byte data to a string
	token := string(tokenBytes)

	// fetch the init image tag
	tag := config.GetAnnotation("qtap-tag")

	// maintains the default of a nil security context (which is equivalent to accepting the pod setting)
	var securityContext *corev1.SecurityContext = nil

	// if the UID and/or GID annotations were set then try to convert them to the correct format for the security context
	if uid, gid := config.GetAnnotation("qtap-uid"), config.GetAnnotation("qtap-gid"); uid != "" || gid != "" {
		var qtapUid int64 = math.MinInt64 // this isn't a permitted UID value and so it is used as not set
		var qtapGid int64 = math.MinInt64 // this isn't a permitted GID value and so it is used as not set

		if uid != "" {
			if n, err := strconv.ParseInt(uid, 10, 64); err == nil {
				qtapUid = n
			}
		}
		if gid != "" {
			if n, err := strconv.ParseInt(gid, 10, 64); err == nil {
				qtapGid = n
			}
		}

		// If a UID was set via annotations we need a security context for the container with the UID
		// and/or GID
		if qtapUid != math.MinInt64 || qtapGid != math.MinInt64 {
			securityContext = &corev1.SecurityContext{} // create empty security context

			// the UID was set, set RunAsUser
			if qtapUid != math.MinInt64 {
				securityContext.RunAsUser = &qtapUid
			}

			// the GID was set, set RunAsGroup
			if qtapGid != math.MinInt64 {
				securityContext.RunAsGroup = &qtapGid
			}
		}
	}

	// create an init container
	qtapContainer := corev1.Container{
		Name:  "qtap",
		Image: fmt.Sprintf("%s:%s", QTAP_IMAGE, tag),
		Args:  []string{"gateway"},
		Env: []corev1.EnvVar{
			{
				Name:  "TOKEN",
				Value: token,
			},
		},
		SecurityContext: securityContext,
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.IntOrString{
						IntVal: 8080,
					},
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       5,
			TimeoutSeconds:      2,
			SuccessThreshold:    1,
			FailureThreshold:    20,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.IntOrString{
						IntVal: 8080,
					},
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       5,
			TimeoutSeconds:      2,
			SuccessThreshold:    1,
			FailureThreshold:    1,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{
						IntVal: 8080,
					},
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       10,
			TimeoutSeconds:      2,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		},
	}

	// LOG_LEVEL
	if logLevel := config.GetAnnotation("log-level"); logLevel != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "LOG_LEVEL",
			Value: logLevel,
		})
	}

	// LOG_ENCODING
	if logEncoding := config.GetAnnotation("log-encoding"); logEncoding != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "LOG_ENCODING",
			Value: logEncoding,
		})
	}

	// LOG_CALLER
	if logCaller := config.GetAnnotation("log-caller"); logCaller != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "LOG_CALLER",
			Value: logCaller,
		})
	}

	// HTTP_LISTEN
	if httpListen := config.GetAnnotation("http-listen"); httpListen != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "HTTP_LISTEN",
			Value: httpListen,
		})
	}

	// HTTPS_LISTEN
	if httpsListen := config.GetAnnotation("https-listen"); httpsListen != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "HTTPS_LISTEN",
			Value: httpsListen,
		})
	}

	// TCP_LISTEN
	if tcpListen := config.GetAnnotation("tcp-listen"); tcpListen != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "TCP_LISTEN",
			Value: tcpListen,
		})
	}

	// BLOCK_UNKNOWN
	if blockUnknown := config.GetAnnotation("block-unknown"); blockUnknown != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "BLOCK_UNKNOWN",
			Value: blockUnknown,
		})
	}

	// ENVOY_LOG_LEVEL
	if envoyLogLevel := config.GetAnnotation("envoy-log-level"); envoyLogLevel != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "ENVOY_LOG_LEVEL",
			Value: envoyLogLevel,
		})
	}

	// DNS_LOOKUP_FAMILY
	if dnsLookupFamily := config.GetAnnotation("dns-lookup-family"); dnsLookupFamily != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "DNS_LOOKUP_FAMILY",
			Value: dnsLookupFamily,
		})
	}

	// API_ENDPOINT
	if apiEndpoint := config.GetAnnotation("api-endpoint"); apiEndpoint != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "ENDPOINT",
			Value: apiEndpoint,
		})
	}

	// append to the list
	pod.Spec.Containers = append(pod.Spec.Containers, qtapContainer)

	// gtg
	return nil
}
