/*
Licensed under the BSD 3-Clause License (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://opensource.org/licenses/BSD-3-Clause

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PodBindingWebhookHandler is the main entrypoint to KubeMod's pod binding interception admission webhook.
type PodBindingWebhookHandler struct {
	client  client.Client
	decoder *admission.Decoder
	log     logr.Logger
}

// NewPodBindingWebhookHandler constructs a new core webhook handler.
func NewPodBindingWebhookHandler(manager manager.Manager, log logr.Logger) *PodBindingWebhookHandler {
	return &PodBindingWebhookHandler{
		client: manager.GetClient(),
		log:    log.WithName("podbinding-webhook"),
	}
}

// Handle triggers the main mutating logic.
func (h *PodBindingWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Resource.Resource == "pods" && req.SubResource == "binding" {
		log := h.log.WithValues("request uid", req.UID, "namespace", req.Namespace, "resource", fmt.Sprintf("%v/%v", req.Resource.Resource, req.Name), "operation", req.Operation)

		binding := &unstructured.Unstructured{}

		err := json.Unmarshal(req.Object.Raw, binding)

		if err != nil {
			log.Error(err, "failed to decode webhook pods/binding request object's manifest into JSON")
			return admission.Allowed("failed to decode webhook request object's manifest into JSON")
		}

		podNamespace, podName, nodeName, err := getBindingTargets(binding.UnstructuredContent())

		if err != nil {
			log.Error(err, "failed to obtain target data from pod binding")
			return admission.Allowed("failed to obtain target data from pod binding")
		}

		// Grab the name of the node and update the pod by attaching an admission.kubemod.io/nodename annotation.
		pod := &unstructured.Unstructured{}
		pod.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Kind:    "Pod",
			Version: "v1",
		})

		err = h.client.Get(ctx, client.ObjectKey{Namespace: podNamespace, Name: podName}, pod)

		if err != nil {
			log.Error(err, "failed to obtain pod", "podNamespace", podNamespace, "podName", podName)
			return admission.Allowed("failed to obtain pod")
		}

		annotations := pod.GetAnnotations()

		if annotations == nil {
			annotations = map[string]string{}
		}

		if enableNodeSyntheticRef, ok := annotations["admission.kubemod.io/enable-node-synthetic-ref"]; ok && enableNodeSyntheticRef == "true" {
			annotations["admission.kubemod.io/nodename"] = nodeName

			pod.SetAnnotations(annotations)

			err = h.client.Update(ctx, pod)

			if err != nil {
				log.Error(err, "failed to update pod with annotation admission.kubemod.io/nodename", "podNamespace", podNamespace, "podName", podName)
				return admission.Allowed("failed to obtain pod")
			}
		}
	}

	return admission.Allowed("ok")
}

func getBindingTargets(binding map[string]interface{}) (string, string, string, error) {
	var ok bool
	var podNamespace, podName, nodeName string
	var err error

	podNamespace, ok, err = unstructured.NestedString(binding, "metadata", "namespace")

	if !ok {
		return "", "", "", fmt.Errorf("unable to find metadata.namespace in pod binding manifest")
	}

	if err != nil {
		return "", "", "", fmt.Errorf("failed to obtain metadata.namespace from pod binding manifest: %v", err)
	}

	podName, ok, err = unstructured.NestedString(binding, "metadata", "name")

	if !ok {
		return "", "", "", fmt.Errorf("unable to find metadata.name in pod binding manifest")
	}

	if err != nil {
		return "", "", "", fmt.Errorf("failed to obtain metadata.name from pod binding manifest: %v", err)
	}

	nodeName, ok, err = unstructured.NestedString(binding, "target", "name")

	if !ok {
		return "", "", "", fmt.Errorf("unable to find target.name in pod binding manifest")
	}

	if err != nil {
		return "", "", "", fmt.Errorf("failed to obtain target.name from pod binding manifest: %v", err)
	}

	return podNamespace, podName, nodeName, nil
}
