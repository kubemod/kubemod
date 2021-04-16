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

package util

import (
	"regexp"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
)

var (
	rexValueTemplatePlaceholder = regexp.MustCompile(`\.#(\d+)`)

	// Dangerous functions which can be used to exploit KubeMod's environment.
	blockFuncs = [...]string{
		"env",
		"expandenv",
	}
)

// PreProcessModRuleGoTemplate converts special tokens such as .#0 to .I0
// which are actual properties on the ModRule golang template context.
func PreProcessModRuleGoTemplate(template string) string {
	return rexValueTemplatePlaceholder.ReplaceAllString(template, "(index .SelectKeyParts $1)")
}

// NewTemplate returns an instance of a Go template wired up with the Sprig template functions.
func NewSafeTemplate(templateName string) *template.Template {
	funcMap := sprig.GenericFuncMap()

	// Delete functions which may be used to exploit KubeMod's environment.
	for _, blockFunc := range blockFuncs {
		delete(funcMap, blockFunc)
	}

	return template.New(templateName).Funcs(template.FuncMap(funcMap))
}
