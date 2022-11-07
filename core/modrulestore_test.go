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
	"io/ioutil"
	"path"
	"sort"
	"strings"

	"github.com/kubemod/kubemod/api/v1beta1"
	"github.com/kubemod/kubemod/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

// ********************************************************************
// Test ModRuleStore.CalculatePatch
// ********************************************************************

var _ = Describe("ModRuleStore", func() {
	var (
		rs *ModRuleStore
	)

	BeforeEach(func() {
		testBed := InitializeModRuleStoreTestBed("kubemod-system", GinkgoT())
		rs = testBed.modRuleStore
	})

	modRuleStoreCalculatePatchTableFunction := func(modRuleYAMLFiles []string, resourceFileJSONFile string, expectationFile string) {
		// Load resource JSON.
		resourceJSON, err := ioutil.ReadFile(path.Join("testdata/resources/", resourceFileJSONFile))
		Expect(err).NotTo(HaveOccurred())

		// Load ModRule YAMLs.
		for _, modRuleYAMLFile := range modRuleYAMLFiles {
			modRuleYAML, err := ioutil.ReadFile(path.Join("testdata/modrules/", modRuleYAMLFile))
			Expect(err).NotTo(HaveOccurred())

			modRule := v1beta1.ModRule{}
			err = yaml.Unmarshal(modRuleYAML, &modRule)
			Expect(err).NotTo(HaveOccurred())

			if modRule.Namespace == "" {
				modRule.Namespace = "my-namespace"
			}

			err = rs.Put(&modRule)
			Expect(err).NotTo(HaveOccurred())
		}

		_, patch, err := rs.CalculatePatch("CREATE", "my-namespace", resourceJSON, nil)
		Expect(err).NotTo(HaveOccurred())

		// Sort the patch because the order returned by CalculatePatch is unstable.
		sort.SliceStable(patch, func(i, j int) bool {
			return (patch[i].Operation + patch[i].Path) < (patch[j].Operation + patch[j].Path)
		})

		expectation, err := ioutil.ReadFile(path.Join("testdata/expectations/", expectationFile))
		Expect(err).NotTo(HaveOccurred())

		Expect(fmt.Sprintf("%v", patch)).To(Equal(strings.TrimSpace(string(expectation))))
	}

	modRuleStoreDetermineRejectionsTableFunction := func(modRuleYAMLFiles []string, resourceFileJSONFile string, expectationFile string, rsPutError string) {
		// Load resource JSON.
		resourceJSON, err := ioutil.ReadFile(path.Join("testdata/resources/", resourceFileJSONFile))
		Expect(err).NotTo(HaveOccurred())

		// Unmarshal that JSON.
		jsonv := interface{}(nil)
		err = json.Unmarshal(resourceJSON, &jsonv)
		Expect(err).NotTo(HaveOccurred())

		// Load ModRule YAMLs.
		for _, modRuleYAMLFile := range modRuleYAMLFiles {
			modRuleYAML, err := ioutil.ReadFile(path.Join("testdata/modrules/", modRuleYAMLFile))
			Expect(err).NotTo(HaveOccurred())

			modRule := v1beta1.ModRule{}
			err = yaml.Unmarshal(modRuleYAML, &modRule)
			Expect(err).NotTo(HaveOccurred())

			modRule.Namespace = "my-namespace"

			err = rs.Put(&modRule)

			if rsPutError == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(fmt.Sprintf("%v", err)).To(Equal(rsPutError))
				return
			}
		}

		rejections := rs.DetermineRejections("CREATE", "my-namespace", jsonv, nil)

		expectation, err := ioutil.ReadFile(path.Join("testdata/expectations/", expectationFile))
		Expect(err).NotTo(HaveOccurred())

		Expect(strings.Join(rejections, ", ")).To(Equal(strings.TrimSpace(string(expectation))))
	}

	DescribeTable("CalculatePatch", modRuleStoreCalculatePatchTableFunction,
		Entry("patch-1 on pod-1 should work as expected", []string{"patch/patch-1.yaml"}, "pod-1.json", "patch-1-pod-1.txt"),
		Entry("patch-2 on pod-1 should work as expected", []string{"patch/patch-2.yaml"}, "pod-1.json", "patch-2-pod-1.txt"),
		Entry("patch-3 on pod-1 should work as expected", []string{"patch/patch-3.yaml"}, "pod-1.json", "patch-3-pod-1.txt"),
		Entry("patch-4 on pod-5 should work as expected", []string{"patch/patch-4.yaml"}, "pod-5.json", "patch-4-pod-5.txt"),
		Entry("patch-5 on pod-5 should work as expected", []string{"patch/patch-5.yaml"}, "pod-5.json", "patch-5-pod-5.txt"),
		Entry("patch-6 on pod-5 should work as expected", []string{"patch/patch-6.yaml"}, "pod-5.json", "patch-6-pod-5.txt"),
		Entry("patch-7 on pod-5 should work as expected", []string{"patch/patch-7.yaml"}, "pod-5.json", "patch-7-pod-5.txt"),
		Entry("patch-7 on pod-5 should work as expected", []string{"patch/patch-8.yaml"}, "pod-5.json", "patch-8-pod-5.txt"),
		Entry("patch-9 on pod-1 should work as expected", []string{"patch/patch-9.yaml"}, "pod-1.json", "patch-9-pod-1.txt"),
		Entry("patch-10 on deployment-1 should work as expected", []string{"patch/patch-10.yaml"}, "deployment-1.json", "patch-10-deployment-1.txt"),
		Entry("patch-10 on deployment-2 should work as expected", []string{"patch/patch-10.yaml"}, "deployment-2.json", "patch-10-deployment-2.txt"),
		Entry("patch-10 on deployment-3 should work as expected", []string{"patch/patch-10.yaml"}, "deployment-3.json", "patch-10-deployment-3.txt"),
		Entry("patch-11 on pod-6 should work as expected", []string{"patch/patch-11.yaml"}, "pod-6.json", "patch-11-pod-6.txt"),
		Entry("patch-12 on pod-6 should work as expected", []string{"patch/patch-12.yaml"}, "pod-6.json", "patch-12-pod-6.txt"),
		Entry("patch-13 on deployment-2 should work - ModRule in kubemod-system and targetNamespaceRegex matches", []string{"patch/patch-13.yaml"}, "deployment-2.json", "patch-13-deployment-2.txt"),
		Entry("patch-14 on deployment-2 should not work - ModRule in kubemod-system, missing targetNamespaceRegex", []string{"patch/patch-14.yaml"}, "deployment-2.json", "empty-array.txt"),
		Entry("patch-15 on deployment-2 should not work - ModRule in kubemod-system, empty targetNamespaceRegex", []string{"patch/patch-15.yaml"}, "deployment-2.json", "empty-array.txt"),
		Entry("patch-16 on deployment-4 should work as expected", []string{"patch/patch-16.yaml"}, "deployment-4.json", "patch-16-deployment-4.txt"),
		Entry("patch-17 on deployment-4 should work as expected", []string{"patch/patch-17.yaml"}, "deployment-4.json", "patch-17-deployment-4.txt"),
		Entry("patch-18 on deployment-4 should work as expected", []string{"patch/patch-18.yaml"}, "deployment-4.json", "patch-18-deployment-4.txt"),
		Entry("patch-19 on deployment-4 should work as expected", []string{"patch/patch-19.yaml"}, "deployment-4.json", "patch-19-deployment-4.txt"),
		Entry("patch-20 on deployment-4 should work as expected", []string{"patch/patch-20.yaml"}, "deployment-4.json", "patch-20-deployment-4.txt"),
		Entry("patch-21 on deployment-4 should work as expected", []string{"patch/patch-21.yaml"}, "deployment-4.json", "patch-21-deployment-4.txt"),
		Entry("patch-22 on deployment-2 should work as expected", []string{"patch/patch-22.yaml"}, "deployment-2.json", "patch-22-deployment-2.txt"),
		Entry("patch-23 on deployment-2 should work as expected", []string{"patch/patch-23.yaml"}, "deployment-2.json", "patch-23-deployment-2.txt"),
		Entry("patch-24 on deployment-2 should work as expected", []string{"patch/patch-24.yaml"}, "deployment-2.json", "patch-24-deployment-2.txt"),
		Entry("patch-25 on deployment-2 should work as expected", []string{"patch/patch-25.yaml"}, "deployment-2.json", "patch-25-deployment-2.txt"),
		Entry("patch-26 on deployment-2 should work as expected", []string{"patch/patch-26.yaml"}, "deployment-2.json", "patch-26-deployment-2.txt"),
		Entry("patch-27 on deployment-4 should work as expected", []string{"patch/patch-27.yaml"}, "deployment-4.json", "patch-27-deployment-4.txt"),
		Entry("patch-28 on deployment-4 should work as expected", []string{"patch/patch-28.yaml"}, "deployment-4.json", "patch-28-deployment-4.txt"),
		Entry("simple tiered execution should work as expected", []string{"patch/patch-29.yaml", "patch/patch-30.yaml"}, "deployment-1.json", "patch-29-30-deployment-1.txt"),
		Entry("tiered execution with two ModRules in second tier should work as expected", []string{"patch/patch-29.yaml", "patch/patch-30.yaml", "patch/patch-31.yaml"}, "deployment-1.json", "patch-29-30-31-deployment-1.txt"),
		Entry("tiered execution with three tiers should work as expected", []string{"patch/patch-29.yaml", "patch/patch-30.yaml", "patch/patch-31.yaml", "patch/patch-32.yaml"}, "deployment-1.json", "patch-29-30-31-32-deployment-1.txt"),
	)

	DescribeTable("DetermineRejections", modRuleStoreDetermineRejectionsTableFunction,
		Entry("malicious-service on service-1 should work as expected", []string{"reject/boolean-service-malicious-external-ips-1.yaml"}, "service-1.json", "malicious-reject-service-1.txt", ""),
		Entry("malicious-service on service-2 should work as expected", []string{"reject/boolean-service-malicious-external-ips-1.yaml"}, "service-2.json", "malicious-reject-service-2.txt", ""),
		Entry("malicious-service on service-3 should work as expected", []string{"reject/boolean-service-malicious-external-ips-1.yaml"}, "service-3.json", "malicious-reject-service-3.txt", ""),
		Entry("malicious-service on service-3 should work as expected", []string{"reject/boolean-service-malicious-external-ips-2.yaml"}, "service-3.json", "malicious-reject-service-4.txt", ""),

		Entry("malicious-service on service-1 should work as expected", []string{"reject/boolean-service-malicious-external-ips-3.yaml"}, "service-1.json", "malicious-reject-service-1.txt", ""),
		Entry("malicious-service on service-2 should work as expected", []string{"reject/boolean-service-malicious-external-ips-3.yaml"}, "service-2.json", "malicious-reject-service-2.txt", ""),
		Entry("malicious-service on service-3 should work as expected", []string{"reject/boolean-service-malicious-external-ips-3.yaml"}, "service-3.json", "malicious-reject-service-3.txt", ""),

		Entry("bad rejection message should error appropriately 1", []string{"reject/bad-reject-message-1.yaml"}, "service-3.json", "", "failed to add ModRule to ModRuleStore: template: rejectMessage:1: unclosed action"),
		Entry("bad rejection message should error appropriately 2", []string{"reject/bad-reject-message-2.yaml"}, "service-3.json", "malicious-reject-service-4.txt", ""),
	)
})

// ********************************************************************
// Test ModRuleStore.Put
// ********************************************************************

var _ = Describe("ModRuleStore", func() {

	const (
		namespacePoolSize = 3
		namePoolSize      = 5
	)

	var (
		rs            *ModRuleStore
		namespacePool *util.NamePool
		namePool      *util.NamePool
	)

	BeforeEach(func() {
		namespacePool = util.NewNamePool(namespacePoolSize)
		namePool = util.NewNamePool(namePoolSize)
	})

	When("populated with duplicate ModRules", func() {

		BeforeEach(func() {
			const loopIterations = namespacePoolSize * namePoolSize * 10

			testBed := InitializeModRuleStoreTestBed("kubemod-system", GinkgoT())

			rs = testBed.modRuleStore

			// Reset the name pools before using them.
			namespacePool.ResetIndex()
			namePool.ResetIndex()

			// Populate the store with data.
			// We will loop more times than a number of combinations between the namespace and name pools.
			// This will test the Add vs Update functionality of Put.
			for i := 0; i < loopIterations; i++ {

				err := rs.Put(&v1beta1.ModRule{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespacePool.GetNextName(),
						Name:      namePool.GetNextName(),
					},
					Spec: v1beta1.ModRuleSpec{
						Match: []v1beta1.MatchItem{
							{
								Select: `$.kind == "Pod"`,
							},
							{
								Select: `$.metadata.labels.app =~ "nginx"`,
							},
						},
					},
				})

				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should only store the last ModRule by name/namespace", func() {
			Expect(getTotalModRuleCountFromStats(rs.GetStats())).To(Equal(namespacePoolSize * namePoolSize))
		})

		Context("and then wiped clean", func() {
			BeforeEach(func() {

				const loopIterations = namespacePoolSize * namePoolSize * 10

				// Reset the name pools before using them.
				namespacePool.ResetIndex()
				namePool.ResetIndex()

				// Note that we will be calling Delete more times than necessary.
				// Deleting a non-existent item should not fail.
				for i := 0; i < loopIterations; i++ {
					namespace := namespacePool.GetNextName()
					name := namePool.GetNextName()
					rs.Delete(namespace, name)
				}

			})

			It("should contain no items", func() {
				Expect(len(rs.GetStats())).To(Equal(0))
			})
		})
	})

})

// Given ModRuleStore stats, sum up the total count of modrules.
func getTotalModRuleCountFromStats(modRuleStoreStats map[string]int) int {
	ret := 0

	for _, modRuleCount := range modRuleStoreStats {
		ret += modRuleCount
	}

	return ret
}
