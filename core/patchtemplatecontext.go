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

// PatchTemplateContext is an internal structure which is passed as context to all patch template executions.
type PatchTemplateContext struct {

	// Namespace is the namespace of the resource being patched.
	Namespace string

	// Target hosts the data of the resource being patched.
	Target interface{}
}
