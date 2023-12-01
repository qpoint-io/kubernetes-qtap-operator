package v1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const GATEWAY_ANNOTATIONS_CONFIGMAP = "qtap-operator-gateway-pod-annotations-configmap"
const INJECTION_ANNOTATIONS_CONFIGMAP = "qtap-operator-injection-pod-annotations-configmap"
const NAMESPACE_EGRESS_LABEL = "qpoint-egress"
const NAMESPACE_INJECTION_LABEL = "qpoint-injection"
const POD_EGRESS_ANNOTATION = "qpoint.io/egress"
const POD_INJECTION_LABEL = "sidecar.qpoint.io/inject"
const ENABLED = "enabled"
const DISABLED = "disabled"
const TRUE = "true"
const FALSE = "false"

type Config struct {
	EnabledEgress     bool // Egress routing is enabled
	EnabledInjection  bool // Sidecar injection is enabled
	InjectCa          bool
	Namespace         string
	OperatorNamespace string
	Client            client.Client
	Ctx               context.Context

	annotations map[string]string
}

// Config scenarios:
// a) Egress routing is enabled and gateway is disabled via the namespace label or pod annotation. This means that the egress traffic is being routed to the qtap service running somewhere else in the cluster.
// b) Egress routing is enabled and gateway is enabled via the namespace label or pod annotation. This means that the egress traffic is being routed through the qtap sidecar proxy.
//
// Egress routing is always controlled by the qtap-init container which manipulates iptables rules for routing egress traffic to one of the above qtap setups.

func (c *Config) Init(pod *corev1.Pod) error {
	// first check if the namespace has the label. If it does then assume that egress is enabled
	namespace := &corev1.Namespace{}
	if err := c.Client.Get(c.Ctx, client.ObjectKey{Name: c.Namespace}, namespace); err != nil {
		return fmt.Errorf("fetching namespace '%s' from the api: %w", c.Namespace, err)
	}

	// if the namespace is labeled for egress, then we enable. A pod annotation override will be checked below
	if namespace.Labels[NAMESPACE_EGRESS_LABEL] == ENABLED {
		c.EnabledEgress = true
	} else if namespace.Labels[NAMESPACE_EGRESS_LABEL] == DISABLED {
		c.EnabledEgress = false
	}

	// check to see if an annotation is set on the pod to enable or disable egress while also verifying
	// if it was enabled for the namespace but needs to be disabled for the pod. If the annotation doesn't exist nothing else needs to be checked
	if egress, exists := pod.Annotations[POD_EGRESS_ANNOTATION]; exists {
		if c.EnabledEgress && egress == DISABLED {
			c.EnabledEgress = false
		}

		if !c.EnabledEgress && egress == ENABLED {
			c.EnabledEgress = true
		}
	}

	// if we're enabled
	if c.EnabledEgress {
		configMapName := GATEWAY_ANNOTATIONS_CONFIGMAP

		// if the namespace is labeled for injection, then we enable. A pod annotation override will be checked below
		if namespace.Labels[NAMESPACE_INJECTION_LABEL] == ENABLED {
			c.EnabledInjection = true
			configMapName = INJECTION_ANNOTATIONS_CONFIGMAP
		} else if namespace.Labels[NAMESPACE_INJECTION_LABEL] == DISABLED {
			c.EnabledInjection = false
		}

		// check to see if an label is set on the pod to enable or disable injection while also verifying
		// if it was enabled for the namespace but needs to be disabled for the pod. If the label doesn't exist nothing else needs to be checked
		if inject, exists := pod.Labels[POD_INJECTION_LABEL]; exists {
			if c.EnabledInjection && inject == FALSE {
				c.EnabledInjection = false
			}

			if !c.EnabledInjection && inject == TRUE {
				c.EnabledInjection = true
				configMapName = INJECTION_ANNOTATIONS_CONFIGMAP
			}
		}

		// let's fetch the default settings in the configmap
		configMap := &corev1.ConfigMap{}
		if err := c.Client.Get(c.Ctx, client.ObjectKey{Name: configMapName, Namespace: c.OperatorNamespace}, configMap); err != nil {
			return fmt.Errorf("fetching configmap '%s' at namespace '%s' from the api: %w", configMapName, c.OperatorNamespace, err)
		}

		// unmarshal the data as yaml
		defaultAnnotations := make(map[string]string)
		if err := yaml.Unmarshal([]byte(configMap.Data["annotations.yaml"]), &defaultAnnotations); err != nil {
			return fmt.Errorf("marshaling the configmap data as yaml: %w", err)
		}

		if pod.Annotations == nil {
			// if there are no annotations, just assign the defaults
			pod.Annotations = defaultAnnotations
		} else {
			// let's apply the default annotations to the pod (for transparency to the admin)
			for key, value := range defaultAnnotations {
				if _, exists := pod.Annotations[key]; !exists {
					pod.Annotations[key] = value
				}
			}
		}

		// and store a direct reference to the annotations for config
		c.annotations = pod.Annotations
	}

	// determine if we should inject the certificate authority
	if c.GetAnnotation("inject-ca") == "true" {
		c.InjectCa = true
	}

	return nil
}

func (c *Config) GetAnnotation(key string) string {
	return c.annotations[fmt.Sprintf("qpoint.io/%s", key)]
}
