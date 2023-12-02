package v1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const SERVICE_ANNOTATIONS_CONFIGMAP = "qtap-operator-service-pod-annotations-configmap"
const INJECT_ANNOTATIONS_CONFIGMAP = "qtap-operator-inject-pod-annotations-configmap"
const NAMESPACE_EGRESS_LABEL = "qpoint-egress"
const POD_EGRESS_LABEL = "qpoint.io/egress"

type EgressType string

const (
	EgressType_UNDEFINED EgressType = "undefined"
	EgressType_DISABLED  EgressType = "disabled"
	EgressType_SERVICE   EgressType = "service"
	EgressType_INJECT    EgressType = "inject"
)

type Config struct {
	EgressType        EgressType
	InjectCa          bool
	Namespace         string
	OperatorNamespace string
	Client            client.Client
	Ctx               context.Context

	annotations map[string]string
}

// Config scenarios:
// a) Egress routing is enabled and gateway is disabled via the namespace label or pod label. This means that the egress traffic is being routed to the qtap service running somewhere else in the cluster.
// b) Egress routing is enabled and gateway is enabled via the namespace label or pod label. This means that the egress traffic is being routed through the qtap sidecar proxy.
//
// Egress routing is always controlled by the qtap-init container which manipulates iptables rules for routing egress traffic to one of the above qtap setups.

func (c *Config) Init(pod *corev1.Pod) error {
	// first check if the namespace has the label. If it does then assume that egress is enabled
	namespace := &corev1.Namespace{}
	if err := c.Client.Get(c.Ctx, client.ObjectKey{Name: c.Namespace}, namespace); err != nil {
		return fmt.Errorf("fetching namespace '%s' from the api: %w", c.Namespace, err)
	}

	namespaceEgressType := EgressType_UNDEFINED
	configMapName := ""

	switch v := namespace.Labels[NAMESPACE_EGRESS_LABEL]; EgressType(v) {
	case EgressType_DISABLED:
		c.EgressType = EgressType_DISABLED
		return nil
	case EgressType_SERVICE:
		c.EgressType = EgressType_SERVICE
		namespaceEgressType = EgressType_SERVICE
		configMapName = SERVICE_ANNOTATIONS_CONFIGMAP
	case EgressType_INJECT:
		c.EgressType = EgressType_INJECT
		namespaceEgressType = EgressType_INJECT
		configMapName = INJECT_ANNOTATIONS_CONFIGMAP
	}

	podEgressType := EgressType_UNDEFINED

	// order matters as pods override namespaces

	switch v := pod.Labels[POD_EGRESS_LABEL]; EgressType(v) {
	case EgressType_DISABLED:
		c.EgressType = EgressType_DISABLED
		return nil
	case EgressType_SERVICE:
		c.EgressType = EgressType_SERVICE
		podEgressType = EgressType_SERVICE
		configMapName = SERVICE_ANNOTATIONS_CONFIGMAP
	case EgressType_INJECT:
		c.EgressType = EgressType_INJECT
		podEgressType = EgressType_INJECT
		configMapName = INJECT_ANNOTATIONS_CONFIGMAP
	}

	// egress is undefined for the entire namespace (regardless of what the pod label says) or pod and thus return immediately
	if namespaceEgressType == EgressType_UNDEFINED && podEgressType == EgressType_UNDEFINED {
		c.EgressType = EgressType_UNDEFINED
		return nil
	}

	if configMapName != "" {
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
