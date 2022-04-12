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
	"strconv"
	"strings"
	"text/template"

	"sigs.k8s.io/yaml"

	"github.com/PaesslerAG/gval"
	evanjsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"github.com/kubemod/kubemod/api/v1beta1"
	"github.com/kubemod/kubemod/jsonpath"
	"github.com/kubemod/kubemod/util"
)

// ModRuleStoreItem wraps around a ModRule and holds a cache of the ModRule's
// match queries, regexp expressions and value templates in compiled form.
type ModRuleStoreItem struct {
	modRule                      *v1beta1.ModRule
	compiledTargetNamespaceRegex *regexp.Regexp
	compiledMatchSelects         map[*v1beta1.MatchItem]gval.Evaluable
	compiledRegexes              map[*v1beta1.MatchItem]*regexp.Regexp
	compiledJSONPatch            []*compiledJSONPatchOperation
	rejectMessageTemplate        *template.Template
	log                          logr.Logger
}

// ModRuleStoreItemFactory is used to construct ModRuleStoreItems.
type ModRuleStoreItemFactory struct {
	jsonPathLanguage *gval.Language
	log              logr.Logger
}

// compiledJSONPatchOperation stored a JSON patch operation with a pre-compiled select JSON Path and go template.
type compiledJSONPatchOperation struct {
	op                  v1beta1.PatchOperationType
	patchSelect         gval.Evaluable
	path                string
	pathSprintfTemplate string
	valueTemplate       *template.Template
}

// patchPathItem is used by the patch calculation logic.
type patchPathItem struct {
	path           string
	selectKeyParts []interface{}
	selectedItem   interface{}
}

var (
	rexSelectKeySegment        = regexp.MustCompile(`\["([^\[\]"]+)"\]`)
	rexPathTemplatePlaceholder = regexp.MustCompile(`#(\d+)`)
)

// NewModRuleStoreItemFactory constructs a new ModRuleStoreItem factory.
func NewModRuleStoreItemFactory(jsonPathLanguage *gval.Language, log logr.Logger) *ModRuleStoreItemFactory {
	return &ModRuleStoreItemFactory{
		jsonPathLanguage: jsonPathLanguage,
		log:              log.WithName("core"),
	}
}

// NewModRuleStoreItem constructs a new ModRule store item.
func (f *ModRuleStoreItemFactory) NewModRuleStoreItem(modRule *v1beta1.ModRule) (*ModRuleStoreItem, error) {
	var err error
	var compiledTargetNamespaceRegex *regexp.Regexp
	if modRule.Spec.TargetNamespaceRegex != nil && *modRule.Spec.TargetNamespaceRegex != "" {
		compiledTargetNamespaceRegex, err = regexp.Compile(*modRule.Spec.TargetNamespaceRegex)
	}

	if err != nil {
		return nil, err
	}

	compiledMatchSelects, err := newCompiledMatchSelects(modRule.Spec.Match, f.jsonPathLanguage)

	if err != nil {
		return nil, err
	}

	compiledRegexes, err := newCompiledRegexes(modRule.Spec.Match)

	if err != nil {
		return nil, err
	}

	compiledJSONPatch, err := newCompiledJSONPatch(modRule.Spec.Patch, f.jsonPathLanguage)

	if err != nil {
		return nil, err
	}

	var rejectMessageTemplate *template.Template

	if modRule.Spec.RejectMessage != nil {
		rejectMessageTemplate, err = util.NewSafeTemplate("rejectMessage").Parse(*modRule.Spec.RejectMessage)
		if err != nil {
			return nil, err
		}
	}

	return &ModRuleStoreItem{
			modRule:                      modRule,
			log:                          f.log,
			compiledTargetNamespaceRegex: compiledTargetNamespaceRegex,
			compiledMatchSelects:         compiledMatchSelects,
			compiledRegexes:              compiledRegexes,
			compiledJSONPatch:            compiledJSONPatch,
			rejectMessageTemplate:        rejectMessageTemplate,
		},
		nil
}

// Given a slice of matchItems, construct a cache of compiled gval expressions.
// Note that the cache is a map which uses the pointers to the match slice elements as keys.
func newCompiledMatchSelects(matchItems []v1beta1.MatchItem, jsonPathLanguage *gval.Language) (map[*v1beta1.MatchItem]gval.Evaluable, error) {
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
func newCompiledJSONPatch(patch []v1beta1.PatchOperation, jsonPathLanguage *gval.Language) ([]*compiledJSONPatchOperation, error) {
	var err error
	var compiledPatch = []*compiledJSONPatchOperation{}

	for _, po := range patch {
		// Default to JSON "null" value in case po.Value is nil.
		var value string = "null"
		var patchSelect gval.Evaluable = nil

		if po.Value != nil {
			value = *po.Value
		}

		// Compile the patch select if any.
		if po.Select != nil {
			selectExpression := fmt.Sprintf(`{#: %s}`, *po.Select)
			patchSelect, err = jsonPathLanguage.NewEvaluable(selectExpression)

			if err != nil {
				return nil, err
			}
		}

		// If the path contains placeholders such as #0 and #1, we need to convert them to Sprintf template.
		pathSprintfTemplate := pathTemplateToSprintfTemplate(po.Path)

		// Compile the go template value.
		tpl, err := util.NewSafeTemplate(po.Path).Parse(util.PreProcessModRuleGoTemplate(value))

		if err != nil {
			return nil, err
		}

		compiledPatch = append(compiledPatch, &compiledJSONPatchOperation{
			op:                  po.Operation,
			patchSelect:         patchSelect,
			path:                po.Path,
			pathSprintfTemplate: pathSprintfTemplate,
			valueTemplate:       tpl,
		})
	}

	return compiledPatch, nil
}

// calculatePatch runs the patch templates and returns a list of patch operations.
func (si *ModRuleStoreItem) calculatePatch(templateContext *PatchTemplateContext, jsonv interface{}, operationLog logr.Logger) (evanjsonpatch.Patch, error) {
	var log logr.Logger
	var operationIndex = 0

	// If we are getting operation-specific log, use it, otherwise, use the singleton log we have for the ModRuleStore item.
	if operationLog != nil {
		log = operationLog.WithName("core")
	} else {
		log = si.log
	}

	b := strings.Builder{}

	b.WriteRune('[')

	for _, cop := range si.compiledJSONPatch {
		// Calculate the actual set of patches to be performed.
		// If there is no select expression, just create a single operation with the given path.
		// If there is a select provided, execute the select query and generate one patch operation per result
		// using the paths generated by gval.
		pathItems := []patchPathItem{}

		if cop.patchSelect != nil {
			result, err := cop.patchSelect(context.Background(), jsonv)

			if err != nil {
				return nil, err
			}

			for key, val := range result.(map[string]interface{}) {
				selectKeyParts := keyPartsFromSelectKey(key)
				path := pathFromKeyParts(selectKeyParts, cop.pathSprintfTemplate)

				if strings.Contains(path, "(BADINDEX)") {
					return nil, fmt.Errorf("failed to generate Patch path from path template \"%v\": generated value \"%v\" ", cop.path, path)
				}

				pathItems = append(pathItems, patchPathItem{
					path:           path,
					selectKeyParts: selectKeyParts,
					selectedItem:   val,
				})
			}
		} else {
			// No select expression? Add a simple path with no select key parts.
			pathItems = append(pathItems, patchPathItem{
				path:           cop.path,
				selectKeyParts: []interface{}{},
				selectedItem:   nil,
			})
		}

		for _, pathItem := range pathItems {

			vb := strings.Builder{}

			// Bake in the select-key parts and selected item into the template context.
			templateContext.SelectKeyParts = pathItem.selectKeyParts
			templateContext.SelectedItem = pathItem.selectedItem

			err := cop.valueTemplate.Execute(&vb, templateContext)

			if err != nil {
				return nil, err
			}

			// Here's the magic - convert the result to a JSON value.
			jsonValue, err := modRuleValueToJSONValue(vb.String())

			if err != nil {
				return nil, err
			}

			if operationIndex > 0 {
				b.WriteRune(',')
			}

			fmt.Fprintf(&b, `{"op": "%v", "path": "%v", "value": %v}`, cop.op, pathItem.path, jsonValue)

			operationIndex++
		}
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

// Given a select key in the form of $["0"]["0"]["3"], and pathSprintfTemplate in the form of
// /abc/%[1]v/def/%[2]v/xyz produce a path by replacing %[i]v with the value of the given key.
func pathFromKeyParts(selectKeyParts []interface{}, pathSprintfTemplate string) string {
	ret := fmt.Sprintf(pathSprintfTemplate, selectKeyParts...)
	ret = strings.ReplaceAll(ret, "%!v(BADINDEX)", "#(BADINDEX)")

	// In case we have passed in more keys than needed, remove the %!(EXTRA part inserted by fmt.Sprintf.
	extraIndex := strings.Index(ret, "%!(EXTRA")

	if extraIndex != -1 {
		ret = ret[0:extraIndex]
	}

	return ret
}

// Convert a select key in the form of $["0"]["abc"]["3"], to a golang slice [0, "abc", 3]
func keyPartsFromSelectKey(selectKey string) []interface{} {
	return extractMatches(selectKey, rexSelectKeySegment, true)
}

// Convert a pathTemplate in the form of "/abc/#0/xyz/#1/bcd" to a
// fmt.Sprintf format of the form /abc/%[1]v/xyz/%[2]v/bcd
func pathTemplateToSprintfTemplate(pathTemplate string) string {
	// Extract the values of the placeholder indices.
	matches := extractMatches(pathTemplate, rexPathTemplatePlaceholder, false)

	// Convert these values to integers and add 1 as we need to switch from zero-based to one-based
	// placeholders (Sprintf works with one-based indices).
	for i := range matches {
		val, _ := strconv.Atoi(matches[i].(string))
		matches[i] = strconv.Itoa(val + 1)
	}

	sprintfTemplate := rexPathTemplatePlaceholder.ReplaceAllString(pathTemplate, `%%[%v]v`)
	return fmt.Sprintf(sprintfTemplate, matches...)
}

// Given a string and a regex, run the regex and extract all strings captured by is capturing groups
// in a slice of strings.
// If typeDetection is true, the function analyzes the contents of the extraction and converts strings to integers if they look like integers
// otherwise it keeps them as strings.
func extractMatches(source string, rex *regexp.Regexp, typeDetection bool) []interface{} {
	matches := rex.FindAllStringSubmatch(source, -1)

	ret := []interface{}{}

	for _, match := range matches {
		if typeDetection {
			if num, err := strconv.Atoi(match[1]); err == nil {
				ret = append(ret, num)
			} else {
				ret = append(ret, match[1])
			}
		} else {
			ret = append(ret, match[1])
		}
	}

	return ret
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
	matchSelect := si.compiledMatchSelects[matchItem]

	result, err := matchSelect(context.Background(), jsonv)
	if err != nil {
		// There is at least one valid reason to be here - when the query tries to match a missing key
		// such as metadata.label.missing_key.
		// In this case we only want to log a DBG message and negate the query.
		si.log.V(1).Info("JSONPath query expression failure", "select", matchItem.Select, "error", err)

		return matchItem.Negate
	}

	var expressionValues []interface{}
	resultArrayContainedUndefined := false

	// Test the result and return negative match if we got nothing back from the query.
	switch vresult := result.(type) {

	// If the query evaluates to nil, this is a no-match.
	case nil:
		return matchItem.Negate

	// If the query evaluates to undefined, this is a no-match.
	case jsonpath.UndefinedType:
		return matchItem.Negate

	// If the query itself returns a boolean, use that as the match.
	// The query value will not be evaluated against match.matchValue, match.matchValues and match.matchRegex.
	case bool:
		return vresult != matchItem.Negate

	// If the query returns an array, filter out all undefined values and then evaluate its contents against match.matchValue, match.matchValues and match.matchRegex.
	case []interface{}:
		// Prep an empty array where we'll put the non-undefined results.
		expressionValues = []interface{}{}

		// Filter out all undefined values.
		for i := range vresult {
			if !jsonpath.IsUndefined(vresult[i]) {
				expressionValues = append(expressionValues, vresult[i])
			}
		}

		// We'll need this flag when we evaluate matchFor = All
		resultArrayContainedUndefined = (len(expressionValues) != len(vresult))

	// If the query returns any type of value other than the ones above, evaluate that value against match.matchValue, match.matchValues and match.matchRegex.
	case interface{}:
		expressionValues = []interface{}{vresult}

	default:
		return matchItem.Negate
	}

	if len(expressionValues) == 0 {
		return matchItem.Negate
	}

	// MatchFor = All, but we had undefined items in the result? That's a no-match.
	if matchItem.MatchFor == v1beta1.MatchForTypeAll && resultArrayContainedUndefined {
		return matchItem.Negate
	}

	// Pre-extract the match regex if any - we don't want to waste cycles picking it up on every iteration over the expressionValues.
	// Note that if the match contains no regex, matchRegexp will be nil
	matchRegexp := si.compiledRegexes[matchItem]

	var ret bool

	// Prepare circuit breaker default return value for the type of match comparison we are running.
	if matchItem.MatchFor == v1beta1.MatchForTypeAny || matchItem.MatchFor == "" {
		ret = matchItem.Negate
	} else {
		ret = !matchItem.Negate
	}

	// Check the resulting values against the criteria values.
	for _, expressionValue := range expressionValues {
		var ev *string

		switch v := expressionValue.(type) {
		case string:
			ev = &v
		default:
			vstr := fmt.Sprintf("%v", v)
			ev = &vstr
		}

		if matchItem.MatchFor == v1beta1.MatchForTypeAny || matchItem.MatchFor == "" {
			if isStringMatch(matchItem, matchRegexp, ev) {
				ret = !matchItem.Negate
				break
			}
		} else {
			if !isStringMatch(matchItem, matchRegexp, ev) {
				ret = matchItem.Negate
				break
			}
		}
	}

	return ret
}

func isStringMatch(matchItem *v1beta1.MatchItem, matchRegexp *regexp.Regexp, value *string) bool {
	if matchItem.MatchValue != nil {
		if *value == *matchItem.MatchValue {
			return true
		}
		// MatchItem has a spec value, but it doesn't match - return negative match.
		return false
	}

	if matchItem.MatchValues != nil && len(matchItem.MatchValues) > 0 {
		for i := range matchItem.MatchValues {
			if *value == matchItem.MatchValues[i] {
				return true
			}
		}

		// MatchItem has spec matchValues, but none of them match - return negative match.
		return false
	}

	if matchItem.MatchRegex != nil {
		if matchRegexp.MatchString(*value) {
			return true
		}

		// MatchItem has a matchRegex, but it does not match - return negative match.
		return false
	}

	// MatchItem has no spec matchValue, matchValues or matchRegex, but the query yielded a value.
	// This is a positive match.
	return true
}
