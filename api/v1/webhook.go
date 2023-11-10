package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Webhook struct {
	Namespace   string
	ApiClient   client.Client
	Decoder     *admission.Decoder
	Development bool
}

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.kb.io,sideEffects=None,admissionReviewVersions=v1

func (w *Webhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	// create a logger
	webhookLog := ctrl.Log.WithName(fmt.Sprintf("pod.v1.admission.webhook[%s]", req.UID))

	pod := &corev1.Pod{}
	err := w.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	webhookLog.Info("Pod mutation requested")

	// initilize a config with defaults
	config := &Config{
		Namespace:         req.Namespace,
		OperatorNamespace: w.Namespace,
		Enabled:           false,
		InjectCa:          false,
		Client:            w.ApiClient,
		Ctx:               ctx,
	}

	// initialize config for this pod
	if err := config.Init(pod); err != nil {
		webhookLog.Error(err, "failed to initialize config for pod")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if config.Enabled {
		webhookLog.Info("Qpoint egress enabled, mutating...")

		// mutate the pod to include egress through the gateway
		if err := MutateEgress(pod, config); err != nil {
			webhookLog.Error(err, "failed to mutate pod for egress")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if config.InjectCa {
			if err := EnsureAssetsInNamespace(config); err != nil {
				webhookLog.Error(err, "failed to add assets to namespace for ca injection")
				return admission.Errored(http.StatusInternalServerError, err)
			}

			if err := MutateCaInjection(pod, config); err != nil {
				webhookLog.Error(err, "failed to mutate pod for ca injection")
				return admission.Errored(http.StatusInternalServerError, err)
			}
		}

	} else {
		webhookLog.Info("Qpoint egress not enabled, ignoring...")
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}
