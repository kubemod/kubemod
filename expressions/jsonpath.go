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

package expressions

import (
	"fmt"

	"github.com/PaesslerAG/gval"
	"github.com/PaesslerAG/jsonpath"
)

// NewJSONPathLanguage constructs the gval language used for the JSONPath match query.
func NewJSONPathLanguage() *gval.Language {
	// Initialize the JSONPath gval language.
	language := gval.Full(
		// Extend the language with a length function which allows expressions such as "length($.spec.containers)"
		gval.Function("length", lengthGValFunction),
		jsonpath.PlaceholderExtension())
	return &language
}

// gval function to add support for length().
// The function works with slices, maps and strings.
func lengthGValFunction(arguments ...interface{}) (interface{}, error) {
	if len(arguments) != 1 {
		return nil, fmt.Errorf("length() expects exactly one array, string or object argument")
	}

	switch v := arguments[0].(type) {
	case nil:
		return 0, nil
	case []interface{}:
		return len(v), nil
	case string:
		return len(v), nil
	case map[string]interface{}:
		return len(v), nil
	}

	return nil, fmt.Errorf("length() expects exactly one array, string or object argument")
}
