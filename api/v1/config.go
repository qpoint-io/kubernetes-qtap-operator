package v1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const ANNOTATIONS_CONFIGMAP = "qtap-operator-default-pod-annotations-configmap"

type Config struct {
	Enabled           bool
	InjectCa          bool
	Namespace         string
	OperatorNamespace string
	Client            client.Client
	Ctx               context.Context

	annotations map[string]string
}

func (c *Config) Init(pod *corev1.Pod) error {
	// first check if the namespace has the label. If it does then assume that egress is enabled
	namespace := &corev1.Namespace{}
	if err := c.Client.Get(c.Ctx, client.ObjectKey{Name: c.Namespace}, namespace); err != nil {
		return fmt.Errorf("fetching namespace '%s' from the api: %w", c.Namespace, err)
	}

	// if the namespace is labeled, then we enable. A pod annotation override will be checked below
	if namespace.Labels["qpoint-egress"] == "enabled" {
		c.Enabled = true
	}

	// check to see if an annotation is set on the pod to enable or disable egress while also verifying
	// if it was enabled for the namespace but needs to be disabled for the pod
	egress, exists := pod.Annotations["qpoint.io/egress"]

	// if the annotation doesn't exist nothing else needs to be checked
	if exists {
		if c.Enabled && egress != "enabled" {
			c.Enabled = false
		}

		if !c.Enabled && egress == "enabled" {
			c.Enabled = true
		}
	}

	// if we're enabled
	if c.Enabled {
		// let's fetch the default settings in the configmap
		configMap := &corev1.ConfigMap{}
		if err := c.Client.Get(c.Ctx, client.ObjectKey{Name: ANNOTATIONS_CONFIGMAP, Namespace: c.OperatorNamespace}, configMap); err != nil {
			return fmt.Errorf("fetching configmap '%s' at namespace '%s' from the api: %w", ANNOTATIONS_CONFIGMAP, c.OperatorNamespace, err)
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
	if c.Get("inject-ca") == "true" {
		c.InjectCa = true
	}

	return nil
}

func (c *Config) Get(key string) string {
	return c.annotations[fmt.Sprintf("qpoint.io/%s", key)]
}
