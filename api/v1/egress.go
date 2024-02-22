package v1

import (
	"fmt"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const INIT_IMAGE = "us-docker.pkg.dev/qpoint-edge/public/kubernetes-qtap-init"
const QTAP_IMAGE = "us-docker.pkg.dev/qpoint-edge/public/qtap"

func MutateEgress(pod *corev1.Pod, config *Config) error {
	// fetch the init image tag
	tag := config.GetAnnotation("qtap-init-tag")

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
			// for the pod. Sometimes it also requires privileged depending on the
			// security within the cluster. See annotations below which allow for
			// setting the running user and group and other settings.
		},
	}

	// SecurityContext RunAsUser
	if runAsUser := config.GetAnnotation("qtap-init-run-as-user"); runAsUser != "" {
		i, err := strconv.ParseInt(runAsUser, 10, 64)
		if err != nil {
			return fmt.Errorf("conversion error: %w", err)
		}
		initContainer.SecurityContext.RunAsUser = &i
	}

	// SecurityContext RunAsGroup
	if runAsGroup := config.GetAnnotation("qtap-init-run-as-group"); runAsGroup != "" {
		i, err := strconv.ParseInt(runAsGroup, 10, 64)
		if err != nil {
			return fmt.Errorf("conversion error: %w", err)
		}
		initContainer.SecurityContext.RunAsGroup = &i
	}

	// SecurityContext RunAsNonRoot
	if runAsNonRoot := config.GetAnnotation("qtap-init-run-as-non-root"); runAsNonRoot != "" {
		b, err := strconv.ParseBool(runAsNonRoot)
		if err != nil {
			return fmt.Errorf("conversion error: %w", err)
		}
		initContainer.SecurityContext.RunAsNonRoot = &b
	}

	// SecurityContext Privileged
	if privileged := config.GetAnnotation("qtap-init-run-as-privileged"); privileged != "" {
		b, err := strconv.ParseBool(privileged)
		if err != nil {
			return fmt.Errorf("conversion error: %w", err)
		}
		initContainer.SecurityContext.Privileged = &b
	}

	// TO_ADDR
	if toAddr := config.GetAnnotation("qtap-init-egress-to-addr"); toAddr != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "TO_ADDR",
			Value: toAddr,
		})
	}

	// TO_DOMAIN
	if toDomain := config.GetAnnotation("qtap-init-egress-to-domain"); toDomain != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "TO_DOMAIN",
			Value: toDomain,
		})
	}

	// PORT_MAPPING
	if portMapping := config.GetAnnotation("qtap-init-egress-port-mapping"); portMapping != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "PORT_MAPPING",
			Value: portMapping,
		})
	}

	// ACCEPT_UIDS
	if acceptUids := config.GetAnnotation("qtap-init-egress-accept-uids"); acceptUids != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "ACCEPT_UIDS",
			Value: acceptUids,
		})
	}

	// ACCEPT_GIDS
	if acceptGids := config.GetAnnotation("qtap-init-egress-accept-gids"); acceptGids != "" {
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
	pod.Spec.InitContainers = append([]corev1.Container{initContainer}, pod.Spec.InitContainers...)

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

	statusListen := config.GetAnnotation("qtap-status-listen")
	var statusPort int32 = 10001
	if statusListen != "" {
		if _, port, err := net.SplitHostPort(statusListen); err == nil {
			portInt, err := strconv.ParseInt(port, 0, 16)
			if err != nil {
				return fmt.Errorf("invalid port: %w", err)
			}
			statusPort = int32(portInt)
		}
	}

	// create an qtap container
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
						IntVal: statusPort,
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
						IntVal: statusPort,
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
						IntVal: statusPort,
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
	if logLevel := config.GetAnnotation("qtap-log-level"); logLevel != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "LOG_LEVEL",
			Value: logLevel,
		})
	}

	// LOG_ENCODING
	if logEncoding := config.GetAnnotation("qtap-log-encoding"); logEncoding != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "LOG_ENCODING",
			Value: logEncoding,
		})
	}

	// LOG_CALLER
	if logCaller := config.GetAnnotation("qtap-log-caller"); logCaller != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "LOG_CALLER",
			Value: logCaller,
		})
	}

	// HTTP_LISTEN
	if httpListen := config.GetAnnotation("qtap-egress-http-listen"); httpListen != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "EGRESS_HTTP_LISTEN",
			Value: httpListen,
		})
	}

	// HTTPS_LISTEN
	if httpsListen := config.GetAnnotation("qtap-egress-https-listen"); httpsListen != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "EGRESS_HTTPS_LISTEN",
			Value: httpsListen,
		})
	}

	// STATUS_LISTEN
	// The annotation was already read above as it is needed to determine the Kubernetes probe port
	if statusListen != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "STATUS_LISTEN",
			Value: statusListen,
		})
	}

	// BLOCK_UNKNOWN
	if blockUnknown := config.GetAnnotation("qtap-block-unknown"); blockUnknown != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "BLOCK_UNKNOWN",
			Value: blockUnknown,
		})
	}

	// ENVOY_LOG_LEVEL
	if envoyLogLevel := config.GetAnnotation("qtap-envoy-log-level"); envoyLogLevel != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "ENVOY_LOG_LEVEL",
			Value: envoyLogLevel,
		})
	}

	// DNS_LOOKUP_FAMILY
	if dnsLookupFamily := config.GetAnnotation("qtap-dns-lookup-family"); dnsLookupFamily != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "DNS_LOOKUP_FAMILY",
			Value: dnsLookupFamily,
		})
	}

	// API_ENDPOINT
	if apiEndpoint := config.GetAnnotation("qtap-api-endpoint"); apiEndpoint != "" {
		qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
			Name:  "ENDPOINT",
			Value: apiEndpoint,
		})
	}

	// by default the pods namespace is always added as a tag. This slice is used for appending
	// additional tags below
	tags := []string{strings.Join([]string{"namespace", pod.Namespace}, ":")}

	//TAGS
	if tagsFilters := config.GetAnnotation("qtap-labels-tags-filter"); tagsFilters != "" {
		// the filter is a list of regular expressions used to determine if labels should be added
		// as tags to qtap

		regexps := []*regexp.Regexp{}

		// loop over the comma separated list of regular expressions and compile a regular expression
		// list that will be used to compare against the labels
		for _, filter := range strings.Split(tagsFilters, ",") {
			if regexFilter, err := regexp.Compile(filter); err != nil {
				return fmt.Errorf("invalid regular expression for tags filter: %w", err)
			} else {
				regexps = append(regexps, regexFilter)
			}
		}

		// loop over all pod labels and if key that matches a regular expression then append it to the
		// list of tags
		for k, v := range pod.Labels {
			for _, r := range regexps {
				if r.MatchString(k) {
					tags = append(tags, strings.Join([]string{k, v}, ":"))
				}
			}
		}
	}

	qtapContainer.Env = append(qtapContainer.Env, corev1.EnvVar{
		Name:  "TAGS",
		Value: strings.Join(tags, ","),
	})

	// append to the list
	pod.Spec.Containers = append([]corev1.Container{qtapContainer}, pod.Spec.Containers...)

	// gtg
	return nil
}
