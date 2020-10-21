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
	"sync"

	evanjsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"github.com/kubemod/kubemod/api/v1beta1"
	ctrljsonpatch "gomodules.xyz/jsonpatch/v2"
)

// ModRuleStore is a thread-safe collection of ModRules organized by namespaces.
type ModRuleStore struct {
	modRuleListMap map[string][]*ModRuleStoreItem
	itemFactory    *ModRuleStoreItemFactory
	rwLock         sync.RWMutex
}

var (
	jsonPatchApplyOptions = &evanjsonpatch.ApplyOptions{
		AccumulatedCopySizeLimit: 0,
		AllowMissingPathOnRemove: true,
		SupportNegativeIndices:   true,
	}
)

// NewModRuleStore instantiates a new ModRuleStore.
func NewModRuleStore(itemFactory *ModRuleStoreItemFactory) *ModRuleStore {
	return &ModRuleStore{
		modRuleListMap: make(map[string][]*ModRuleStoreItem),
		itemFactory:    itemFactory,
		rwLock:         sync.RWMutex{},
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
func (s *ModRuleStore) getMatchingModRuleStoreItems(namespace string, jsonv interface{}) []*ModRuleStoreItem {
	modRules := []*ModRuleStoreItem{}

	if namespaceModRules, ok := s.modRuleListMap[namespace]; ok {
		for _, mrsi := range namespaceModRules {
			if mrsi.IsMatch(jsonv) {
				modRules = append(modRules, mrsi)
			}
		}
	}

	return modRules
}

// CalculatePatch calculates the set of patch operations to apply against a given resource
// based on the ModRules matchins the resource.
func (s *ModRuleStore) CalculatePatch(namespace string, originalJSON []byte, operationLog logr.Logger) ([]ctrljsonpatch.JsonPatchOperation, error) {
	var modifiedJSON = originalJSON

	jsonv := interface{}(nil)
	err := json.Unmarshal(originalJSON, &jsonv)

	templateContext := PatchTemplateContext{
		Namespace: namespace,
		Target:    &jsonv,
	}

	if err != nil {
		return nil, err
	}

	// Find all matching rules.
	matchingModRules := s.getMatchingModRuleStoreItems(namespace, jsonv)

	// Apply the patches of each matching rule.
	for _, mrsi := range matchingModRules {
		epatch, err := mrsi.calculatePatch(&templateContext, jsonv, operationLog)

		if err != nil {
			return nil, err
		}

		modifiedJSON, err = epatch.ApplyWithOptions(modifiedJSON, jsonPatchApplyOptions)

		if err != nil {
			return nil, err
		}
	}

	// Calculate the final mutation.
	return ctrljsonpatch.CreatePatch(originalJSON, modifiedJSON)
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
