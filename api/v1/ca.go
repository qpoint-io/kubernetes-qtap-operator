package v1

import (
	_ "embed"

	corev1 "k8s.io/api/core/v1"
)

//go:embed assets/build-ca.sh
var buildCaScript string

func MutateCaInjection(pod *corev1.Pod, config *Config) error {
	return nil
}
