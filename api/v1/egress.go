package v1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const INIT_IMAGE = "us-docker.pkg.dev/qpoint-edge/public/kubernetes-qtap-init"

func MutateEgress(pod *corev1.Pod, config *Config) error {
	// fetch the init image tag
	tag := config.Get("egress-init-tag")

	// create an init container
	initContainer := corev1.Container{
		Name:  "qtap-init",
		Image: fmt.Sprintf("%s:%s", INIT_IMAGE, tag),
		Env:   []corev1.EnvVar{},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
		},
	}

	// TO_ADDR
	toAddr := config.Get("egress-to-addr")
	if toAddr != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "TO_ADDR",
			Value: toAddr,
		})
	}

	// TO_DOMAIN
	toDomain := config.Get("egress-to-domain")
	if toAddr == "" && toDomain != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "TO_DOMAIN",
			Value: toDomain,
		})
	}

	// PORT_MAPPING
	portMapping := config.Get("egress-port-mapping")
	if portMapping != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "PORT_MAPPING",
			Value: portMapping,
		})
	}

	// ACCEPT_UIDS
	acceptUids := config.Get("egress-accept-uids")
	if acceptUids != "" {
		initContainer.Env = append(initContainer.Env, corev1.EnvVar{
			Name:  "ACCEPT_UIDS",
			Value: acceptUids,
		})
	}

	// ACCEPT_GIDS
	acceptGids := config.Get("egress-accept-gids")
	if acceptGids != "" {
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
