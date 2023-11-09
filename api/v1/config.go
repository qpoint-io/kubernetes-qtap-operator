package v1

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	Enabled  bool
	InjectCa bool

	apiClient client.Client
}

func InitConfig(apiClient client.Client, namespace string, pod *corev1.Pod) (*Config, error) {
	// start with a default config
	config := &Config{
		Enabled:   false,
		InjectCa:  true,
		apiClient: apiClient,
	}

	// enable for time-being
	config.Enabled = true

	return config, nil
}
