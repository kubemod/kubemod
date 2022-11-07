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
	"fmt"
	"github.com/kubemod/kubemod/api/v1beta1"
	"strings"

	"github.com/go-logr/logr"
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
		log:          log.WithName("modrule-webhook"),
		modRuleStore: modRuleStore,
	}
}

// Handle trigers the main mutating logic.
func (h *DragnetWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := h.log.WithValues("request uid", req.UID, "namespace", req.Namespace, "resource", fmt.Sprintf("%v/%v", req.Resource.Resource, req.Name), "operation", req.Operation)

	// if the request target operation is a deletion the received object will be nil
	// in order to allow rejections based on the object we therefore have to
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

	patchedJSON, patch, err := h.modRuleStore.CalculatePatch(v1beta1.ModRuleOperation(req.Operation), storeNamespace, obj, log)

	if err != nil {
		log.Error(err, "Failed to calculate patch")
		// We don't want to fail the admission just because we cannot decode the JSON.
		return admission.Allowed("failed to calculate patch")
	}

	// Then test the result against the set of relevant Reject rules.
	rejections := h.modRuleStore.DetermineRejections(v1beta1.ModRuleOperation(req.Operation), storeNamespace, patchedJSON, log)

	if len(rejections) > 0 {
		rejectionMessages := strings.Join(rejections, ",")
		log.Info("Rejected", "rejections", rejectionMessages)
		// We don't want to fail the admission just because someone messed up their Reject rule.
		return admission.Denied(fmt.Sprintf("operation rejected by the following ModRule(s): %s", rejectionMessages))
	}

	// If we are here, then the object and its patch passed all rejection rules.
	// Check if we actually had a patch and if yes, return that to Kubernetes for processing.
	if patch != nil && len(patch) > 0 {
		log.Info("Applying ModRule patch", "patch", patch)
		return admission.Patched("patched ok", patch...)
	}

	return admission.Allowed("non-patched ok")
}
