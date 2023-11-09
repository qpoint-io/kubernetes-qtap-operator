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

var (
	webhookLog = ctrl.Log.WithName("pod.v1.admission.webhook")
)

type Webhook struct {
	Development bool
	ApiClient   client.Client
	Decoder     *admission.Decoder
}

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.kb.io,sideEffects=None,admissionReviewVersions=v1

func (w *Webhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := w.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	webhookLog.Info(fmt.Sprintf("Pod mutation requested: %s", req.UID))

	// initialize config for this pod
	config, err := InitConfig(w.ApiClient, req.Namespace, pod)
	if err != nil {
		webhookLog.Error(err, "failed to initialize config for pod")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if config.Enabled {
		// if w.Development {
		// 	fmt.Println("Before: ")
		// 	jsonBytes, _ := json.MarshalIndent(pod, "", "  ")
		// 	fmt.Println(string(jsonBytes))
		// }

		// mutate the pod to include egress through the gateway
		if err := MutateEgress(pod, config); err != nil {
			webhookLog.Error(err, "failed to mutate pod for egress")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if config.InjectCa {
			if err := MutateCaInjection(pod, config); err != nil {
				webhookLog.Error(err, "failed to mutate pod for ca injection")
				return admission.Errored(http.StatusInternalServerError, err)
			}
		}

		// if w.Development {
		// 	fmt.Println("AFTER: ")
		// 	jsonBytes, _ := json.MarshalIndent(pod, "", "  ")
		// 	fmt.Println(string(jsonBytes))
		// }
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}
