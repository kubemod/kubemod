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

import "regexp"

var (
	rexValueTemplatePlaceholder = regexp.MustCompile(`\.#(\d+)`)
)

// PreProcessModRuleGoTemplate converts special tokens such as .#0 to .I0
// which are actual properties on the ModRule golang template context.
func PreProcessModRuleGoTemplate(template string) string {
	return rexValueTemplatePlaceholder.ReplaceAllString(template, "(index .SelectKeyParts $1)")
}
