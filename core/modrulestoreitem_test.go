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
	"io/ioutil"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	"github.com/kubemod/kubemod/api/v1beta1"
)

var _ = Describe("ModRuleStoreItem", func() {

	var (
		testBed *ModRuleStoreItemTestBed
	)

	BeforeEach(func() {
		testBed = InitializeModRuleStoreItemTestBed(GinkgoT())
	})

	modRuleStoreItemIsMatchTableFunction := func(modRuleYAMLFile string, resourceFileJSONFile string, expectedMatch bool) {
		// Load resource JSON.
		resourceJSON, err := ioutil.ReadFile(path.Join("testdata/resources/", resourceFileJSONFile))
		Expect(err).NotTo(HaveOccurred())

		resource := interface{}(nil)
		err = json.Unmarshal(resourceJSON, &resource)
		Expect(err).NotTo(HaveOccurred())

		// Load ModRule YAML.
		modRuleYAML, err := ioutil.ReadFile(path.Join("testdata/modrules/", modRuleYAMLFile))
		Expect(err).NotTo(HaveOccurred())

		modRule := v1beta1.ModRule{}
		err = yaml.Unmarshal(modRuleYAML, &modRule)
		Expect(err).NotTo(HaveOccurred())

		modRuleStoreItem, err := testBed.itemFactory.NewModRuleStoreItem(&modRule)
		Expect(err).NotTo(HaveOccurred())

		Expect(modRuleStoreItem.IsMatch(resource)).To(Equal(expectedMatch))
	}

	DescribeTable("IsMatch", modRuleStoreItemIsMatchTableFunction,
		Entry("should match when single exact value matches", "reject/exact-pod.yaml", "pod-1.json", true),
		Entry("should fail to match when single exact value fails to match", "reject/exact-pod.yaml", "service-1.json", false),
		Entry("should fail to match when single exact value fails to negative-match", "reject/exact-negative-pod.yaml", "pod-1.json", false),
		Entry("should match when single exact value negative-matches", "reject/exact-negative-pod.yaml", "service-1.json", true),
		Entry("should match when both exact values match", "reject/exact-pod-and-label.yaml", "pod-1.json", true),
		Entry("should fail to match when both exact values fail to match", "reject/exact-pod-and-label.yaml", "service-1.json", false),
		Entry("should fail to match when at least one of exact values match", "reject/exact-pod-and-label.yaml", "pod-2.json", false),
		Entry("should fail to match when positive query matches, but negative query fails to match", "reject/exact-pod-and-negative-label.yaml", "pod-1.json", false),
		Entry("should match when both positive query match and negative queries match", "reject/exact-pod-and-negative-label.yaml", "pod-2.json", true),
		Entry("should match array queries", "reject/exact-pod-container-port.yaml", "pod-1.json", true),
		Entry("should fail to match array queries that don't match", "reject/exact-pod-container-port.yaml", "pod-2.json", false),
		Entry("should fail to match array queries that don't match", "reject/exact-pod-negative-container-port.yaml", "pod-1.json", false),
		Entry("should match array queries that match", "reject/exact-pod-negative-container-port.yaml", "pod-2.json", true),
		Entry("should fail to match queries with missing keys", "reject/exact-missing-container.yaml", "service-1.json", false),
		Entry("should fail to match array queries with array index out of range", "reject/exact-missing-container.yaml", "pod-1.json", false),
		Entry("should match array queries when they negatively match", "reject/exact-negative-missing-container.yaml", "pod-1.json", true),
		Entry("should match array queries when they negatively match", "reject/exact-negative-missing-container.yaml", "pod-2.json", true),
		Entry("should match array queries when they negatively match", "reject/exact-negative-missing-container.yaml", "service-1.json", true),
		Entry("should match query using length", "reject/exact-pod-number-of-containers.yaml", "pod-1.json", true),
		Entry("should fail to match query using length", "reject/exact-pod-number-of-containers.yaml", "pod-2.json", false),

		Entry("should match boolean query using length", "reject/boolean-pod-number-of-containers-greater-than-one.yaml", "pod-1.json", false),
		Entry("should match boolean query using length", "reject/boolean-pod-number-of-containers-greater-than-one.yaml", "pod-2.json", true),
		Entry("should match boolean query using length", "reject/boolean-pod-number-of-containers-greater-than-one.yaml", "service-1.json", false),
		Entry("should match negative boolean query using length", "reject/boolean-pod-negative-number-of-containers-greater-than-one.yaml", "pod-1.json", true),
		Entry("should match negative boolean query using length", "reject/boolean-pod-negative-number-of-containers-greater-than-one.yaml", "pod-2.json", false),
		Entry("should match negative boolean query using length", "reject/boolean-pod-negative-number-of-containers-greater-than-one.yaml", "service-1.json", false),

		Entry("should ignore value when matching boolean queries", "reject/boolean-exact-pod-number-of-containers-greater-than-one.yaml", "pod-1.json", false),
		Entry("should ignore value when matching boolean queries", "reject/boolean-exact-pod-number-of-containers-greater-than-one.yaml", "pod-2.json", true),
		Entry("should ignore value when matching boolean queries", "reject/boolean-exact-pod-number-of-containers-greater-than-one.yaml", "service-1.json", false),

		Entry("should match when query matches one of many values", "reject/one-of-kind.yaml", "pod-1.json", true),
		Entry("should match when query matches one of many values", "reject/one-of-kind.yaml", "deployment-1.json", true),
		Entry("should match when query matches one of many values", "reject/one-of-kind.yaml", "service-1.json", false),

		Entry("should match when multiple queries match one of many values", "reject/one-of-kind-and-one-of-label.yaml", "pod-1.json", true),
		Entry("should match when multiple queries match one of many values", "reject/one-of-kind-and-one-of-label.yaml", "pod-2.json", false),
		Entry("should match when multiple queries match one of many values", "reject/one-of-kind-and-one-of-label.yaml", "deployment-1.json", true),
		Entry("should match when multiple queries match one of many values", "reject/one-of-kind-and-one-of-label.yaml", "service-1.json", false),

		Entry("should match when query negatively matches one of many values", "reject/one-of-kind-negative.yaml", "pod-1.json", false),
		Entry("should match when query negatively matches one of many values", "reject/one-of-kind-negative.yaml", "deployment-1.json", false),
		Entry("should match when query negatively matches one of many values", "reject/one-of-kind-negative.yaml", "service-1.json", true),

		Entry("should match when multiple queries match one of many values", "reject/one-of-kind-and-negative-one-of-label.yaml", "pod-1.json", false),
		Entry("should match when multiple queries match one of many values", "reject/one-of-kind-and-negative-one-of-label.yaml", "pod-2.json", true),
		Entry("should match when multiple queries match one of many values", "reject/one-of-kind-and-negative-one-of-label.yaml", "deployment-1.json", false),
		Entry("should match when multiple queries match one of many values", "reject/one-of-kind-and-negative-one-of-label.yaml", "service-1.json", false),

		Entry("should match negative boolean query", "reject/boolean-pod-negative-run-as-non-root.yaml", "pod-1.json", false),
		Entry("should match negative boolean query", "reject/boolean-pod-negative-run-as-non-root.yaml", "pod-2.json", true),
		Entry("should match negative boolean query", "reject/boolean-pod-negative-run-as-non-root.yaml", "deployment-1.json", false),
		Entry("should match negative boolean query", "reject/boolean-pod-negative-run-as-non-root.yaml", "service-1.json", false),

		Entry("should match predicate query", "reject/select-predicate-pod-port-equal-to-80.yaml", "pod-1.json", true),
		Entry("should match predicate query", "reject/select-predicate-pod-port-equal-to-80.yaml", "pod-2.json", false),
		Entry("should match predicate query", "reject/select-predicate-pod-port-equal-to-80.yaml", "deployment-1.json", false),
		Entry("should match predicate query", "reject/select-predicate-pod-port-equal-to-80.yaml", "service-1.json", false),

		Entry("should match predicate query", "reject/select-predicate-pod-port-equal-to-or-greater-than-80.yaml", "pod-1.json", true),
		Entry("should match predicate query", "reject/select-predicate-pod-port-equal-to-or-greater-than-80.yaml", "pod-2.json", true),
		Entry("should match predicate query", "reject/select-predicate-pod-port-equal-to-or-greater-than-80.yaml", "deployment-1.json", false),
		Entry("should match predicate query", "reject/select-predicate-pod-port-equal-to-or-greater-than-80.yaml", "service-1.json", false),

		Entry("should match deep predicate query", "reject/select-deep-predicate-pod-container-image-regex-port-equal-to-80.yaml", "pod-1.json", true),
		Entry("should match deep predicate query", "reject/select-deep-predicate-pod-container-image-regex-port-equal-to-80.yaml", "pod-2.json", false),
		Entry("should match deep predicate query", "reject/select-deep-predicate-pod-container-image-regex-port-equal-to-80.yaml", "pod-3.json", false),
		Entry("should match deep predicate query", "reject/select-deep-predicate-pod-container-image-regex-port-equal-to-80.yaml", "pod-4.json", false),

		Entry("should match predicate query", "reject/select-predicate-pod-container-image-regex.yaml", "pod-1.json", true),
		Entry("should match predicate query", "reject/select-predicate-pod-container-image-regex.yaml", "pod-2.json", true),
		Entry("should match predicate query", "reject/select-predicate-pod-container-image-regex.yaml", "pod-3.json", false),

		Entry("should match one-of predicate query", "reject/select-one-of-container-image.yaml", "pod-1.json", true),
		Entry("should match one-of predicate query", "reject/select-one-of-container-image.yaml", "pod-2.json", false),
		Entry("should match one-of predicate query", "reject/select-one-of-container-image.yaml", "pod-3.json", true),

		Entry("should match select regex query", "reject/select-regex-container-image.yaml", "pod-1.json", true),
		Entry("should match select regex query", "reject/select-regex-container-image.yaml", "pod-2.json", true),
		Entry("should match select regex query", "reject/select-regex-container-image.yaml", "pod-3.json", false),

		Entry("should match predicate query with regex", "reject/select-predicate-regex-container-image.yaml", "pod-1.json", false),
		Entry("should match predicate query with regex", "reject/select-predicate-regex-container-image.yaml", "pod-2.json", true),
		Entry("should match predicate query with regex", "reject/select-predicate-regex-container-image.yaml", "pod-3.json", false),
	)
})
