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
	"regexp"
	"strings"
	"text/template"

	"sigs.k8s.io/yaml"

	"github.com/PaesslerAG/gval"
	evanjsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"github.com/kubemod/kubemod/api/v1beta1"
)

// ModRuleStoreItem wraps around a ModRule and holds a cache of the ModRule's
// match queries, regexp expressions and value templates in compiled form.
type ModRuleStoreItem struct {
	modRule           *v1beta1.ModRule
	compiledJSONPaths map[*v1beta1.MatchItem]gval.Evaluable
	compiledRegexes   map[*v1beta1.MatchItem]*regexp.Regexp
	compiledJSONPatch []*compiledJSONPatchOperation
	log               logr.Logger
}

// compiledJSONPatchOperation stored a JSON patch operation with a pre-compiled go template.
type compiledJSONPatchOperation struct {
	op            v1beta1.PatchOperationType
	path          string
	valueTemplate *template.Template
}

// ModRuleStoreItemFactory is used to construct ModRuleStoreItems.
type ModRuleStoreItemFactory struct {
	jsonPathLanguage *gval.Language
	log              logr.Logger
}

// NewModRuleStoreItemFactory constructs a new ModRuleStoreItem factory.
func NewModRuleStoreItemFactory(jsonPathLanguage *gval.Language, log logr.Logger) *ModRuleStoreItemFactory {
	return &ModRuleStoreItemFactory{
		jsonPathLanguage: jsonPathLanguage,
		log:              log.WithName("core"),
	}
}

// NewModRuleStoreItem constructs a new ModRule store item.
func (f *ModRuleStoreItemFactory) NewModRuleStoreItem(modRule *v1beta1.ModRule) (*ModRuleStoreItem, error) {
	compiledJSONPaths, err := newCompiledJSONPaths(modRule.Spec.Match, f.jsonPathLanguage)

	if err != nil {
		return nil, err
	}

	compiledRegexes, err := newCompiledRegexes(modRule.Spec.Match)

	if err != nil {
		return nil, err
	}

	compiledJSONPatch, err := newCompiledJSONPatch(modRule.Spec.Patch)

	if err != nil {
		return nil, err
	}

	return &ModRuleStoreItem{
			modRule:           modRule,
			log:               f.log,
			compiledJSONPaths: compiledJSONPaths,
			compiledRegexes:   compiledRegexes,
			compiledJSONPatch: compiledJSONPatch,
		},
		nil
}

// Given a slice of matchItems, construct a cache of compiled gval expressions.
// Note that the cache is a map which uses the pointers to the match slice elements as keys.
func newCompiledJSONPaths(matchItems []v1beta1.MatchItem, jsonPathLanguage *gval.Language) (map[*v1beta1.MatchItem]gval.Evaluable, error) {
	var err error
	cache := make(map[*v1beta1.MatchItem]gval.Evaluable)

	for i := range matchItems {
		cache[&matchItems[i]], err = jsonPathLanguage.NewEvaluable(matchItems[i].Select)

		if err != nil {
			return nil, err
		}
	}

	return cache, nil
}

// Given a slice of matchItems, construct a cache of compiled regexp expressions.
// Note that the cache is a map which uses the pointers to the match slice elements as keys.
func newCompiledRegexes(matchItems []v1beta1.MatchItem) (map[*v1beta1.MatchItem]*regexp.Regexp, error) {
	var err error
	cache := make(map[*v1beta1.MatchItem]*regexp.Regexp)

	for i := range matchItems {
		if matchItems[i].MatchRegex != nil {
			cache[&matchItems[i]], err = regexp.Compile(*matchItems[i].MatchRegex)

			if err != nil {
				return nil, err
			}
		}
	}

	return cache, nil
}

// newCompiledJSONPatch converts ModRule patch to evanphx jsonpatch Patch.
func newCompiledJSONPatch(patch []v1beta1.PatchOperation) ([]*compiledJSONPatchOperation, error) {
	var compiledPatch = []*compiledJSONPatchOperation{}

	for _, po := range patch {
		// Default to JSON "null" value in case po.Value is nil.
		value := "null"

		if po.Value != nil {
			value = *po.Value
		}

		tpl, err := template.New(po.Path).Parse(value)

		if err != nil {
			return nil, err
		}

		compiledPatch = append(compiledPatch, &compiledJSONPatchOperation{
			op:            po.Operation,
			path:          po.Path,
			valueTemplate: tpl,
		})
	}

	return compiledPatch, nil
}

// calculatePatch runs the patch templates and returns a list of patch operations.
func (si *ModRuleStoreItem) calculatePatch(templateContext *PatchTemplateContext, operationLog logr.Logger) (evanjsonpatch.Patch, error) {
	var log logr.Logger

	// If we are getting operation-specific log, use it, otherwise, use the singleton log we have for the ModRuleStore item.
	if operationLog != nil {
		log = operationLog.WithName("core")
	} else {
		log = si.log
	}

	b := strings.Builder{}

	b.WriteRune('[')

	for index, cop := range si.compiledJSONPatch {
		vb := strings.Builder{}

		err := cop.valueTemplate.Execute(&vb, templateContext)

		if err != nil {
			return nil, err
		}

		// Here's the magic - convert the result to a JSON value.
		jsonValue, err := modRuleValueToJSONValue(vb.String())

		if err != nil {
			return nil, err
		}

		if index > 0 {
			b.WriteRune(',')
		}

		fmt.Fprintf(&b, `{"op": "%v", "path": "%v", "value": %v}`, cop.op, cop.path, jsonValue)
	}

	b.WriteRune(']')

	patchText := b.String()
	epatch, err := evanjsonpatch.DecodePatch([]byte(patchText))

	if err != nil {
		log.Error(err, "invalid JSON patch text", "patch text", patchText)
	} else {
		log.V(1).Info("modrule patch", "modrule", si.modRule.GetNamespacedName(), "patch", epatch)
	}

	return epatch, err
}

// modRuleValueToJSONValue converts a ModRule value to a JSON value.
// modRuleValueToJSONValue performs some analysis of the ModRule value in order to infer its JSON type:
// - If the value matches the format of a JavaScript number, it is considered to be a number.
// - If the value matches a boolean literal (true/false), it is considered to be a boolean literal.
// - If the value matches 'null', it is considered to be null.
// - If the value is surrounded by double-quotes, it is considered to be a string.
// - If the value is surrounded by brackets, it is considered to be a JSON array.
// - If the value is surrounded by curly braces, it is considered to be a JSON object.
// - If none of the above is true, the value is considered to be a string.
func modRuleValueToJSONValue(modRuleValue string) (string, error) {
	jsonb, err := yaml.YAMLToJSON([]byte(modRuleValue))
	return string(jsonb), err
}

// IsMatch runs all the queries stored in the receiving store item against the given JSON object.
// If all of the queries match, it returns true, otherwise, returns false.
func (si *ModRuleStoreItem) IsMatch(jsonv interface{}) bool {
	matchItems := si.modRule.Spec.Match

	for i := range matchItems {
		matchItem := &matchItems[i]

		if !si.isMatch(matchItem, jsonv) {
			return false
		}
	}

	return true
}

func (si *ModRuleStoreItem) isMatch(matchItem *v1beta1.MatchItem, jsonv interface{}) bool {

	jsonPath := si.compiledJSONPaths[matchItem]

	result, err := jsonPath(context.Background(), jsonv)
	if err != nil {
		// There is at least one valid reason to be here - when the query tries to match a missing key
		// such as metadata. label.missing_key.
		// In this case we only want to log a DBG info message and negate the query.
		si.log.V(1).Info("JSONPath query expression failure", "select", matchItem.Select, "error", err)

		return matchItem.Negative
	}

	var expressionValues []interface{}

	// Test the result and return negative match if we got nothing back from the query.
	switch vresult := result.(type) {

	// If the query evaluates to nil, this is a no-match.
	case nil:
		return matchItem.Negative

	// If the query itself returns a boolean, use that as the match.
	// The query value will not be evaluated against match.matchValue, match.matchValues and match.matchRegex.
	case bool:
		return vresult != matchItem.Negative

	// If the query returns an array, evaluate its contents against match.matchValue, match.matchValues and match.matchRegex.
	case []interface{}:
		expressionValues = vresult

	// If the query returns any type of value other than the ones above, evaluate that value against match.matchValue, match.matchValues and match.matchRegex.
	case interface{}:
		expressionValues = []interface{}{vresult}

	default:
		return matchItem.Negative
	}

	if len(expressionValues) == 0 {
		return matchItem.Negative
	}

	// Pre-extract the match regex if any - we don't want to waste cycles picking it up on every iteration over the expressionValues.
	// Note that if the match contains no regex, matchRegexp will be nil
	matchRegexp := si.compiledRegexes[matchItem]

	// Check if any of the resulting values match any of the criteria values.
	for _, expressionValue := range expressionValues {
		var ev *string

		switch v := expressionValue.(type) {
		case string:
			ev = &v
		default:
			vstr := fmt.Sprintf("%v", v)
			ev = &vstr
		}

		// We have a positive match? No need to look further.
		if isStringMatch(matchItem, matchRegexp, ev) == !matchItem.Negative {
			return !matchItem.Negative
		}
	}

	return matchItem.Negative
}

func isStringMatch(matchItem *v1beta1.MatchItem, matchRegexp *regexp.Regexp, value *string) bool {
	if matchItem.MatchValue != nil {
		if *value == *matchItem.MatchValue {
			return !matchItem.Negative
		}
		// MatchItem has a spec value, but it doesn't match - return negative match.
		return matchItem.Negative
	}

	if matchItem.MatchValues != nil && len(matchItem.MatchValues) > 0 {
		for i := range matchItem.MatchValues {
			if *value == matchItem.MatchValues[i] {
				return !matchItem.Negative
			}
		}

		// MatchItem has spec matchValues, but none of them match - return negative match.
		return matchItem.Negative
	}

	if matchItem.MatchRegex != nil {
		if matchRegexp.MatchString(*value) {
			return !matchItem.Negative
		}

		// MatchItem has a matchRegex, but it does not match - return negative match.
		return matchItem.Negative
	}

	// MatchItem has no spec matchValue, matchValues or matchRegex, but the query yielded a value.
	// This is a positive match.
	return !matchItem.Negative
}
