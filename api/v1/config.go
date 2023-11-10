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
	// check to see if an annotation is set on the pod to enable egress
	egress, exists := pod.Annotations["qpoint.io/egress"]
	if exists && egress == "enabled" {
		c.Enabled = true
	}

	// if we're not enabled yet, let's check the namespace
	if !c.Enabled {
		namespace := &corev1.Namespace{}
		if err := c.Client.Get(c.Ctx, client.ObjectKey{Name: c.Namespace}, namespace); err != nil {
			return fmt.Errorf("fetching namespace '%s' from the api: %w", c.Namespace, err)
		}

		// if the namespace is labeled, then we enable
		if namespace.Labels["qpoint-egress"] == "enabled" {
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

		// let's apply the default annotations to the pod (for transparency to the admin)
		for key, value := range defaultAnnotations {
			if _, exists := pod.Annotations[key]; !exists {
				pod.Annotations[key] = value
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
