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

package v1beta1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ModRuleSpec defines the desired state of ModRule
type ModRuleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Type describes the type of a ModRule.
	// Valid values are:
	// - "Patch" - the rule performs modifications on all the matching resources as they are created.
	// - "Reject" - the rule rejects the creation of all matching resources.
	Type ModRuleType `json:"type"`

	// ExecutionTier is a value between -32767 and 32766.
	// ExecutionTier controls when this ModRule will be executed as it relates to the other ModRules loaded in the system.
	// ModRules are matched and executed in tiers, starting with the lowest tier.
	// The results of executing all ModRules in a tier are passed as input to the ModRules in the next tier.
	// This cascading execution continues until the highest tier of ModRules has been executed.
	// ModRules in the same tier are executed in indeterminate order.
	// +optional
	// +kubebuilder:default=0
	ExecutionTier int16 `json:"executionTier"`

	// Operation describes the operation of a ModRule.
	// Valid values are:
	// - "CREATE" - the rule performs modifications on all the matching resources as they are created.
	// - "UPDATE" - the rule performs modifications on all the matching resources as they are updated.
	// - "DELETE" - the rule performs modifications on all the matching resources as they are deleted.
	// +optional
	Operation []ModRuleOperation `json:"operation"`

	// Match is a list of match items which consist of select queries and expected match values or regular expressions.
	// When all match items for an object are positive, the rule is in effect.
	// +kubebuilder:validation:MinItems=1
	Match []MatchItem `json:"match"`

	// Patch is a list of patch operations to perform on the matching resources at the time of creation.
	// The value part of a patch operation can be a golang template which accepts the resource as its context.
	// This field must be provided for ModRules of type "patch"
	// +optional
	Patch []PatchOperation `json:"patch,omitempty"`

	// RejectMessage is an optional message displayed when a resource is rejected by a Reject ModRule.
	// The field is a Golang template evaluated in the context of the object being rejected.
	// +optional
	RejectMessage *string `json:"rejectMessage,omitempty"`

	// TargetNamespaceRegex is optional and only applies to ModRules in "kubemod-system" namespace.
	// Its usage enables cluster-wide matching of namespaced resources.
	TargetNamespaceRegex *string `json:"targetNamespaceRegex,omitempty"`
}

// MatchItem represents a single match query.
type MatchItem struct {
	// Select is a JSONPath query expression: https://goessner.net/articles/JsonPath/ which yields zero or more values.
	// If no match value or regex is specified, if the query yields a non-empty result, the match is considered positive.
	Select string `json:"select"`

	// MatchFor instructs how to match the results against the match... requirements.
	// Valid values are:
	// - "Any" - the match is considered positive if any of the results of select have a match.
	// - "All" - the match is considered positive only if all of the results of select have a match.
	// +optional
	MatchFor MatchForType `json:"matchFor,omitempty"`

	// MatchValue specifies the exact value to match the result of Select by.
	// The match is considered positive if at least one of the results of evaluating the select query yields a match when compared to matchValue.
	// +nullable
	MatchValue *string `json:"matchValue,omitempty"`

	// MatchValues specifies a list of values to match the result of Select by.
	// The match is considered positive if at least one of the results of evaluating the select query yields a match when compared to any of the values in the array.
	// +optional
	MatchValues []string `json:"matchValues,omitempty"`

	// MatchRegex specifies the regular expression to compare the result of Select by.
	// The match is considered positive if at least one of the results of evaluating the select query yields a match when compared to value.
	// +nullable
	MatchRegex *string `json:"matchRegex,omitempty"`

	// Negate indicates whether the match result should be to inverted.
	// Defaults to false.
	// +optional
	Negate bool `json:"negate,omitempty"`
}

// PatchOperation represents a single JSON Patch operation.
type PatchOperation struct {

	// Operation is the type of JSON Path operation to perform against the target element.
	Operation PatchOperationType `json:"op"`

	// Optional JSONPath query expression: https://goessner.net/articles/JsonPath/ used to construct path.
	// A patch operation is created for each result of the query.
	// A placeholder is created for each wildcard and filter in the expression.
	// These placeholders can be used when constructing "path".
	// For example, if select is "$.spec.containers[*].ports[?@.containerPort == 80]"
	// placeholder #0 will point to the index of "containers" and #1 will point to the index of "ports".
	// This allows us to define paths such as "/spec/template/spec/containers/#0/securityContext"
	Select *string `json:"select,omitempty"`

	// Path is the JSON path to the target element.
	Path string `json:"path"`

	// Value is the JSON representation of the modification.
	// The value is a golang template which is evaluated against the context of the target resource.
	// KubeMod performs some analysis of the result of the template evaluation in order to infer its JSON type:
	// - If the value matches the format of a JavaScript number, it is considered to be a number.
	// - If the value matches a boolean literal (true/false), it is considered to be a boolean literal.
	// - If the value matches 'null', it is considered to be null.
	// - If the value is surrounded by double-quotes, it is considered to be a string.
	// - If the value is surrounded by brackets, it is considered to be a JSON array.
	// - If the value is surrounded by curly braces, it is considered to be a JSON object.
	// - If none of the above is true, the value is considered to be a string.
	// +nullable
	Value *string `json:"value,omitempty"`
}

// PatchOperationType describes the type of a JSON Patch operation.
// Only one of the following ModRule types may be specified.
// +kubebuilder:validation:Enum=add;replace;remove
type PatchOperationType string

const (
	// Add represents an "add" JSON Patch operation.
	Add PatchOperationType = "add"
	// Replace represents a "replace" JSON Patch operation.
	Replace PatchOperationType = "replace"
	// Remove represents a "remove" JSON Patch operation.
	Remove PatchOperationType = "remove"
)

// ModRuleType describes the type of a ModRule.
// Only one of the following ModRule types may be specified.
// +kubebuilder:validation:Enum=Patch;Reject
type ModRuleType string

// ModRuleOperation describes the operation a ModRule is executed on.
// Only the following ModRule operations may be specified.
// +kubebuilder:validation:Enum=CREATE;UPDATE;DELETE
type ModRuleOperation string

const (
	// ModRuleTypePatch describes a ModRule which performs modifications on the target resource.
	ModRuleTypePatch ModRuleType = "Patch"

	// ModRuleTypeReject indicates that the ModRule should reject Create events for resources which match the rule.
	ModRuleTypeReject ModRuleType = "Reject"
)

// MatchForType describes the type of a match.
// Only one of the following ModRule types may be specified.
// +kubebuilder:validation:Enum=Any;All
type MatchForType string

const (
	// MatchForTypeAny indicates that a match is positive when any of the selected results matches any of the match requirements.
	MatchForTypeAny MatchForType = "Any"
	// MatchForTypeAll indicates that a match is positive when all of the selected results matches any of the match requirements.
	MatchForTypeAll MatchForType = "All"
)

// ModRuleStatus defines the observed state of ModRule
type ModRuleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// ModRule is the Schema for the modrules API
type ModRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModRuleSpec   `json:"spec,omitempty"`
	Status ModRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ModRuleList contains a list of ModRule
type ModRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ModRule{}, &ModRuleList{})
}

// GetNamespacedName returns a combined namespace/name.
func (m *ModRule) GetNamespacedName() string {
	return fmt.Sprintf("%s/%s", m.Namespace, m.Name)
}
