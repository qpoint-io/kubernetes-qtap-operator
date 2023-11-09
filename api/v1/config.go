package v1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	defaultAnnotations = map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
)

type Config struct {
	Namespace string
	Enabled   bool
	InjectCa  bool

	apiClient   client.Client
	annotations map[string]string
}

func (c *Config) Init(ctx context.Context, pod *corev1.Pod) error {
	// check to see if an annotation is set on the pod to enable egress
	egress, exists := pod.Annotations["qpoint.io/egress"]
	if exists && egress == "enabled" {
		c.Enabled = true
	}

	// if we're not enabled yet, let's check the namespace
	if !c.Enabled {
		namespace := &corev1.Namespace{}
		if err := c.apiClient.Get(ctx, client.ObjectKey{Name: c.Namespace}, namespace); err != nil {
			return fmt.Errorf("fetching namespace '%s' from the api: %w", c.Namespace, err)
		}

		// if the namespace is labeled, then we enable
		if namespace.Labels["qpoint-egress"] == "enabled" {
			c.Enabled = true
		}
	}

	// if we're enabled
	if c.Enabled {

		// let's apply the default annotations to the pod (for transparency to the admin)
		for key, value := range defaultAnnotations {
			if _, exists := pod.Annotations[key]; !exists {
				pod.Annotations[key] = value
			}
		}

		// and store a direct reference to the annotations for config
		c.annotations = pod.Annotations
	}

	return nil
}

func (c *Config) Get(key string) string {
	return c.annotations[fmt.Sprintf("qpoint.io/%s", key)]
}
