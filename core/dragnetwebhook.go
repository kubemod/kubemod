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
	"strings"

	"github.com/kubemod/kubemod/api/v1beta1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// DragnetWebhookHandler is the main entrypoint to KubeMod's mutating admission webhook.
type DragnetWebhookHandler struct {
	client       client.Client
	decoder      *admission.Decoder
	log          logr.Logger
	modRuleStore *ModRuleStore
}

// NewDragnetWebhookHandler constructs a new core webhook handler.
func NewDragnetWebhookHandler(manager manager.Manager, modRuleStore *ModRuleStore, log logr.Logger) *DragnetWebhookHandler {
	return &DragnetWebhookHandler{
		client:       manager.GetClient(),
		log:          log.WithName("dragnet-webhook"),
		modRuleStore: modRuleStore,
	}
}

// Handle triggers the main mutating logic.
func (h *DragnetWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := h.log.WithValues("request uid", req.UID, "namespace", req.Namespace, "resource", fmt.Sprintf("%v/%v", req.Resource.Resource, req.Name), "operation", req.Operation)

	// If the request target operation is a deletion, the received object will be nil.
	// We therefore use the object being deleted as the target passed to the ModRule calculations.
	var obj []byte
	if req.Operation != "DELETE" {
		obj = req.Object.Raw
	} else {
		obj = req.OldObject.Raw
	}

	storeNamespace := req.Namespace
	// If the target object is a namespace, UPDATE operations will pass in the namespace itself as the owner of the namespace.
	// This is misleading - namespaces are cluster-wide objects.
	// Set the storeNamespace to an empty string to reflect that and pick up the cluster-wide modrules.
	if req.Kind.Group == "" && req.Kind.Version == "v1" && req.Kind.Kind == "Namespace" {
		storeNamespace = ""
	}

	// Inject syntheticRefs into object.
	obj, err := h.injectSyntheticRefs(ctx, obj, storeNamespace)

	if err != nil {
		log.Error(err, "Failed to inject syntheticRefs into object manifest")
		return admission.Allowed("failed to inject syntheticRefs into object manifest")
	}

	// First run patch operations.
	patchedJSON, patch, err := h.modRuleStore.CalculatePatch(v1beta1.ModRuleAdmissionOperation(req.Operation), storeNamespace, obj, log)

	if err != nil {
		log.Error(err, "Failed to calculate patch")
		// We don't want to fail the admission just because we cannot decode the JSON.
		return admission.Allowed("failed to calculate patch")
	}

	// Then test the result against the set of relevant Reject rules.
	rejections := h.modRuleStore.DetermineRejections(v1beta1.ModRuleAdmissionOperation(req.Operation), storeNamespace, patchedJSON, log)

	if len(rejections) > 0 {
		rejectionMessages := strings.Join(rejections, ",")
		log.Info("Rejected", "rejections", rejectionMessages)
		// We don't want to fail the admission just because someone messed up their Reject rule.
		return admission.Denied(fmt.Sprintf("operation rejected by the following ModRule(s): %s", rejectionMessages))
	}

	// If we are here, then the object and its patch passed all rejection rules.
	// Check if we actually had a patch and if yes, return that to Kubernetes for processing.
	if len(patch) > 0 {
		log.Info("Applying ModRule patch", "patch", patch)
		return admission.Patched("patched ok", patch...)
	}

	return admission.Allowed("non-patched ok")
}

func (h *DragnetWebhookHandler) injectSyntheticRefs(ctx context.Context, originalJSON []byte, namespace string) ([]byte, error) {
	obj := &unstructured.Unstructured{}
	syntheticRefs := make(map[string]interface{})
	var err error

	// If the target is a namespaced object, grab the namespace from the manager's client.
	if namespace != "" {
		err = h.injectNamespaceSyntheticRef(ctx, namespace, syntheticRefs)
		if err != nil {
			return nil, fmt.Errorf("failed to inject namespace synthetic ref: %v", err)
		}
	}

	err = json.Unmarshal(originalJSON, obj)

	if err != nil {
		return nil, fmt.Errorf("failed to decode webhook request object's manifest into JSON: %v", err)
	}

	// If the target object is a pod, check if the pod has a node name set by KubeMod in annotation ref.kubemod.io/nodename
	// and if yes, inject the pod's node manifest into the synthetic refs.
	if isPod(obj.UnstructuredContent()) {
		nodeName, ok, err := unstructured.NestedString(obj.UnstructuredContent(), "metadata", "annotations", "ref.kubemod.io/nodename")

		if ok && err == nil && nodeName != "" {
			err = h.injectPodNodeSyntheticRef(ctx, nodeName, syntheticRefs)

			if err != nil {
				return nil, fmt.Errorf("failed to inject node synthetic ref: %v", err)
			}
		}
	}

	// Set KubeMod syntheticRefs field.
	obj.UnstructuredContent()["syntheticRefs"] = syntheticRefs

	// Remove verbose managedFields as it is useless for the purposes of modrules,
	// but at the same time it's quite verbose and potentially slows down modrule processing.
	obj.SetManagedFields(nil)

	return json.Marshal(obj)
}

func isPod(obj map[string]interface{}) bool {
	kind, ok, err := unstructured.NestedString(obj, "kind")

	if ok && err == nil && kind == "Pod" {
		return true
	}

	return false
}

func (h *DragnetWebhookHandler) injectNamespaceSyntheticRef(ctx context.Context, namespace string, syntheticRefs map[string]interface{}) error {
	ns := &unstructured.Unstructured{}
	ns.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    "Namespace",
		Version: "v1",
	})

	err := h.client.Get(ctx, client.ObjectKey{Name: namespace}, ns)

	if err != nil {
		return fmt.Errorf("failed to retrieve namespace '%s' : %v", namespace, err)
	}

	// Remove managedFields - we don't need this and it only litters the namespace manifest.
	ns.SetManagedFields(nil)
	// Remove annotation kubectl.kubernetes.io/last-applied-configuration as it simply duplicates the namespace manifest.
	annotations := ns.GetAnnotations()
	delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
	ns.SetAnnotations(annotations)

	syntheticRefs["namespace"] = ns
	return nil
}

func (h *DragnetWebhookHandler) injectPodNodeSyntheticRef(ctx context.Context, nodeName string, syntheticRefs map[string]interface{}) error {
	node := &unstructured.Unstructured{}

	node.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    "Node",
		Version: "v1",
	})

	err := h.client.Get(ctx, client.ObjectKey{Name: nodeName}, node)

	if err != nil {
		return fmt.Errorf("failed to retrieve node '%s': %v", nodeName, err)
	}

	// Remove managedFields - we don't need this and it only litters the manifest.
	node.SetManagedFields(nil)

	syntheticRefs["node"] = node
	return nil
}
