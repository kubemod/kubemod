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
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	evanjsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"github.com/kubemod/kubemod/api/v1beta1"
	ctrljsonpatch "gomodules.xyz/jsonpatch/v2"
)

// ClusterModRulesNamespace is a type of string used by DI to inject the namespace where cluster-wide ModRules are deployed.
type ClusterModRulesNamespace string

// ModRuleStore is a thread-safe collection of ModRules organized by namespaces.
type ModRuleStore struct {
	modRuleListMap           map[string][]*ModRuleStoreItem
	itemFactory              *ModRuleStoreItemFactory
	clusterModRulesNamespace string
	rwLock                   sync.RWMutex
	log                      logr.Logger
}

var (
	jsonPatchApplyOptions = &evanjsonpatch.ApplyOptions{
		AccumulatedCopySizeLimit: 0,
		AllowMissingPathOnRemove: true,
		SupportNegativeIndices:   true,
		EnsurePathExistsOnAdd:    true,
	}
)

// NewModRuleStore instantiates a new ModRuleStore.
func NewModRuleStore(itemFactory *ModRuleStoreItemFactory, clusterModRulesNamespace ClusterModRulesNamespace, log logr.Logger) *ModRuleStore {
	return &ModRuleStore{
		modRuleListMap:           make(map[string][]*ModRuleStoreItem),
		itemFactory:              itemFactory,
		clusterModRulesNamespace: string(clusterModRulesNamespace),
		rwLock:                   sync.RWMutex{},
		log:                      log.WithName("core"),
	}
}

// Put adds or updates a mod rule to the rule set.
// ModRules are identified by their namespace/name pair.
func (s *ModRuleStore) Put(modRule *v1beta1.ModRule) error {
	var namespace = modRule.Namespace
	var namespaceModRules []*ModRuleStoreItem
	var existingModRuleIndex int
	var ok bool

	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	if namespaceModRules, ok = s.modRuleListMap[namespace]; !ok {
		// This is the first time we see this namespace - create the namespaced slice of modrules.
		namespaceModRules = []*ModRuleStoreItem{}
		s.modRuleListMap[namespace] = namespaceModRules
		// Short-circuit the call to findModRuleIndexByName - we know the new modrule has never been added to the set before.
		existingModRuleIndex = -1
	} else {
		// This modrule may have been added to the nameset slice before.
		// Find its index.
		existingModRuleIndex = findItemIndexByName(namespaceModRules, modRule.Name)
	}

	// Instantiate a store item.
	modRuleStoreItem, err := s.itemFactory.NewModRuleStoreItem(modRule)

	if err != nil {
		return fmt.Errorf("failed to add ModRule to ModRuleStore: %v", err)
	}

	// If a modrule exists with the same name, replace it, otherwise append to the list.
	if existingModRuleIndex != -1 {
		s.modRuleListMap[namespace][existingModRuleIndex] = modRuleStoreItem
	} else {
		s.modRuleListMap[namespace] = append(namespaceModRules, modRuleStoreItem)
	}

	return nil
}

// Delete removes a modrule from the set.
func (s *ModRuleStore) Delete(namespace string, name string) {
	s.rwLock.Lock()
	defer s.rwLock.Unlock()

	if namespaceModRules, ok := s.modRuleListMap[namespace]; ok {
		if modRuleIndex := findItemIndexByName(namespaceModRules, name); modRuleIndex != -1 {
			// Fast delete - breaks the order of elements, but avoids copying the slice.
			// Copy last element into the slot of the one we are deleting, then truncate 1 element from the tail of the slice.
			namespaceModRules[modRuleIndex] = namespaceModRules[len(namespaceModRules)-1]
			namespaceModRules[len(namespaceModRules)-1] = nil
			namespaceModRules = namespaceModRules[:len(namespaceModRules)-1]

			// If the resulting list of modrules is empty, remove the namespace from the map,
			// otherwise, update the namespace in the map with the new list.
			if len(namespaceModRules) == 0 {
				delete(s.modRuleListMap, namespace)
			} else {
				s.modRuleListMap[namespace] = namespaceModRules
			}
		}
	}
}

// getMatchingModRuleStoreItems returns a slice with all the mod rules which match the given unmarshalled JSON.
func (s *ModRuleStore) getMatchingModRuleStoreItems(namespace string, modRuleType v1beta1.ModRuleType, jsonv interface{}) []*ModRuleStoreItem {
	modRules := []*ModRuleStoreItem{}

	potentialRules := []*ModRuleStoreItem{}

	// First look at cluster-wide mod rules.
	// If the resource is a non-namespaced object (its namespace is empty),
	// or the resource's namespace matches the mod rule's TargetNamespaceRegex,
	// then the mod rule is a potential match and subject to further examination by the heavier .IsMatch().
	for _, mrsi := range s.modRuleListMap[s.clusterModRulesNamespace] {
		if (namespace == "" && mrsi.compiledTargetNamespaceRegex == nil) ||
			(mrsi.compiledTargetNamespaceRegex != nil &&
				mrsi.compiledTargetNamespaceRegex.Match([]byte(namespace))) {
			potentialRules = append(potentialRules, mrsi)
		}
	}

	// If the resource is namespaced, add the mod rules deployed to that same namespace
	// to the list of potentially matching mod rules.
	if namespace != "" {
		potentialRules = append(potentialRules, s.modRuleListMap[namespace]...)
	}

	// Perform the actual matching.
	for _, mrsi := range potentialRules {
		if mrsi.modRule.Spec.Type == modRuleType && mrsi.IsMatch(jsonv) {
			modRules = append(modRules, mrsi)
		}
	}

	return modRules
}

// Extract the value of a JSON-unmarshalled object pointed at by a colon-separated path.
func getValueFromJSONObject(jsonv interface{}, path string) interface{} {
	pathComponents := strings.Split(path, ":")
	var node map[string]interface{}
	var val interface{}
	var ok bool

	if node, ok = jsonv.(map[string]interface{}); !ok {
		return nil
	}

	for i, component := range pathComponents {
		if val, ok = node[component]; !ok {
			return nil
		}

		if i < len(pathComponents)-1 {
			if node, ok = val.(map[string]interface{}); !ok {
				return nil
			}
		} else {
			return val
		}
	}

	return nil
}

// Given an unmarshalled JSON, extractLastAppliedConfiguration looks for annotation kubectl.kubernetes.io/last-applied-configuration
// and if it exists, it returns its value, otherwise it returns nil.
func extractLastAppliedConfiguration(jsonv interface{}) []byte {
	config := getValueFromJSONObject(jsonv, "metadata:annotations:kubectl.kubernetes.io/last-applied-configuration")

	if config != nil {
		if bc, ok := config.(string); ok {
			return []byte(bc)
		}
	}

	return nil
}

// CalculatePatch calculates the set of patch operations to apply against a given resource
// based on the ModRules matching the resource.
func (s *ModRuleStore) CalculatePatch(namespace string, originalJSON []byte, operationLog logr.Logger) (interface{}, []ctrljsonpatch.JsonPatchOperation, error) {
	var modifiedJSON = originalJSON
	jsonv := interface{}(nil)
	var log logr.Logger

	// If we are getting operation-specific log, use it, otherwise, use the singleton log we have for the ModRuleStore item.
	if operationLog != nil {
		log = operationLog.WithName("core")
	} else {
		log = s.log
	}

	err := json.Unmarshal(originalJSON, &jsonv)

	if err != nil {
		return nil, nil, err
	}

	// Extract the contents of annotation kubectl.kubernetes.io/last-applied-configuration if available.
	lastAppliedConfigurationJSON := extractLastAppliedConfiguration(jsonv)

	templateContext := PatchTemplateContext{
		Namespace: namespace,
		Target:    &jsonv,
	}

	// Find all matching Patch rules.
	matchingModRules := s.getMatchingModRuleStoreItems(namespace, v1beta1.ModRuleTypePatch, jsonv)

	// Apply the patches of each matching rule.
	for _, mrsi := range matchingModRules {
		epatch, err := mrsi.calculatePatch(&templateContext, jsonv, operationLog)

		// If an error occurred while calculating the patch for a ModRule, simply log it and continue to the next one.
		if err != nil {
			log.Error(err, "failed calculating patch for ModRule", "rule", mrsi.modRule.GetNamespacedName())
			continue
		}

		modifiedJSON, err = epatch.ApplyWithOptions(modifiedJSON, jsonPatchApplyOptions)

		// If an error occurred while applying the patch for a ModRule, simply log it and continue to the next one.
		if err != nil {
			log.Error(err, "failed applying patch for ModRule", "rule", mrsi.modRule.GetNamespacedName())
			continue
		}

		err = json.Unmarshal(modifiedJSON, &jsonv)

		if err != nil {
			return nil, nil, err
		}

		// Apply the same patch to the kubectl last-applied-configuration annotation.
		if lastAppliedConfigurationJSON != nil {
			lastAppliedConfigurationJSON, err = epatch.ApplyWithOptions(lastAppliedConfigurationJSON, jsonPatchApplyOptions)

			// If an error occurred while applying the patch for a ModRule, simply log it and continue to the next one.
			if err != nil {
				log.Error(err, "failed applying patch for ModRule to last-applied-configuration annotation", "rule", mrsi.modRule.GetNamespacedName())
			}
		}
	}

	if lastAppliedConfigurationJSON != nil {
		// Stick the patched last-applied-configuration into the modified document.
		node := jsonv.(map[string]interface{})["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
		node["kubectl.kubernetes.io/last-applied-configuration"] = string(lastAppliedConfigurationJSON)
		modifiedJSON, err = json.Marshal(&jsonv)

		if err != nil {
			return nil, nil, err
		}
	}

	// Calculate the final mutation.
	ops, err := ctrljsonpatch.CreatePatch(originalJSON, modifiedJSON)

	if err != nil {
		return nil, nil, err
	}

	return jsonv, ops, nil
}

// DetermineRejections checks if the given object should be rejected based on the current Reject ModRules stored in the namespace.
func (s *ModRuleStore) DetermineRejections(namespace string, jsonv interface{}, operationLog logr.Logger) []string {
	var rejectionMessages = []string{}
	var log logr.Logger

	// If we are getting operation-specific log, use it, otherwise, use the singleton log we have for the ModRuleStore item.
	if operationLog != nil {
		log = operationLog.WithName("core")
	} else {
		log = s.log
	}

	// Find all matching Reject rules.
	matchingModRules := s.getMatchingModRuleStoreItems(namespace, v1beta1.ModRuleTypeReject, jsonv)

	templateContext := RejectTemplateContext{
		Namespace: namespace,
		Target:    &jsonv,
	}

	// Enumerate all matching reject rules and evaluate their messages.
	for _, mrsi := range matchingModRules {

		if mrsi.rejectMessageTemplate != nil {
			vb := strings.Builder{}
			err := mrsi.rejectMessageTemplate.Execute(&vb, templateContext)

			if err != nil {
				// Log the template error, but do not stop the rejection.
				log.Error(err, "invalid rejectMessage template", "rule", mrsi.modRule.GetNamespacedName(), "rejectMessage text", *mrsi.modRule.Spec.RejectMessage)
				rejectionMessages = append(rejectionMessages, fmt.Sprintf("%s", mrsi.modRule.GetNamespacedName()))
			} else {
				rejectionMessages = append(rejectionMessages, fmt.Sprintf("%s: \"%s\"", mrsi.modRule.GetNamespacedName(), vb.String()))
			}
		} else {
			rejectionMessages = append(rejectionMessages, fmt.Sprintf("%s", mrsi.modRule.GetNamespacedName()))
		}
	}

	return rejectionMessages
}

// findModRuleIndexByName returns the index of the first ModRule which matches the given name
// or -1 if no match is found.
func findItemIndexByName(modRules []*ModRuleStoreItem, name string) int {
	for index, val := range modRules {
		if val.modRule.Name == name {
			return index
		}
	}

	return -1
}

// GetStats returns a map of all namespaces and their modrule count.
// Useful for unit testing.
func (s *ModRuleStore) GetStats() map[string]int {
	namespaces := make(map[string]int)

	s.rwLock.RLock()
	defer s.rwLock.RUnlock()

	for namespace, modruleList := range s.modRuleListMap {
		namespaces[namespace] = len(modruleList)
	}

	return namespaces
}
