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
	"io/ioutil"
	"path"
	"sort"

	"github.com/golang/mock/gomock"
	"github.com/kubemod/kubemod/api/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"
)

var _ = Describe("DragnetWebhookHandler", func() {
	var (
		testBed *DragnetWebhookHandlerTestBed
		handler *DragnetWebhookHandler
	)

	BeforeEach(func() {
		testBed = InitializeDragnetWebhookHandlerTestBed("kubemod-system", GinkgoT())

		handler = &DragnetWebhookHandler{
			client:       testBed.mockK8sClient,
			log:          testBed.log,
			modRuleStore: testBed.modRuleStore,
		}
	})

	AfterEach(func() {
		testBed.mockCtrl.Finish()
	})

	// Load a modrule into the modrule store.
	loadModRule := func(modRuleFile string, namespace string) {
		modRuleYAML, err := ioutil.ReadFile(path.Join("testdata/modrules/", modRuleFile))
		Expect(err).NotTo(HaveOccurred())

		modRule := v1beta1.ModRule{}
		err = yaml.Unmarshal(modRuleYAML, &modRule)
		Expect(err).NotTo(HaveOccurred())

		// Fill out default values for missing properties.
		modRule.Default()

		if modRule.Namespace == "" {
			modRule.Namespace = namespace
		}

		err = testBed.modRuleStore.Put(&modRule)
		Expect(err).NotTo(HaveOccurred())
	}

	It("should be able to insert data derived from a pod's node synthetic reference", func() {
		// Load resource JSON - a pod which has a ref.kubemod.io/nodename annotation (presumably injected by KubeMod's podbinding webhook).
		resourceJSON, err := ioutil.ReadFile(path.Join("testdata/resources/", "pod-7.json"))
		Expect(err).NotTo(HaveOccurred())

		// Load modrule which injects annotations derived from node metadata.
		loadModRule("patch/patch-33.yaml", "my-namespace")

		// Prepare the K8s client mock for a call to get the default namespace manifest.
		testBed.mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: "my-namespace"}, gomock.Any()).Return(nil)

		// Prepare the K8s client mock for a call to get the node manifest which the pod is scheduled to live on.
		testBed.mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: "my-node-1234"}, gomock.Any()).
			DoAndReturn(func(ctx context.Context, key client.ObjectKey, node *unstructured.Unstructured) error {
				// Stamp out the node's region and zone.
				node.SetLabels(map[string]string{
					"topology.kubernetes.io/region": "us-west-2",
					"topology.kubernetes.io/zone":   "us-west-2b",
				})
				return nil
			})

		request := admission.Request{
			AdmissionRequest: admissionv1beta1.AdmissionRequest{
				Namespace: "my-namespace",
				Operation: "UPDATE",
				Object: k8sruntime.RawExtension{
					Raw: resourceJSON,
				},
			},
		}

		response := handler.Handle(context.Background(), request)
		Expect(response).ToNot(BeNil())

		Expect(len(response.Patches)).To(Equal(2))

		// Sort the patch because the order returned by CalculatePatch is unstable.
		sort.SliceStable(response.Patches, func(i, j int) bool {
			return (response.Patches[i].Operation + response.Patches[i].Path) < (response.Patches[j].Operation + response.Patches[j].Path)
		})

		Expect(response.Patches[0].Operation).To(Equal("add"))
		Expect(response.Patches[0].Path).To(Equal("/metadata/labels/topology.kubernetes.io~1region"))
		Expect(response.Patches[0].Value).To(Equal("us-west-2"))

		Expect(response.Patches[1].Operation).To(Equal("add"))
		Expect(response.Patches[1].Path).To(Equal("/metadata/labels/topology.kubernetes.io~1zone"))
		Expect(response.Patches[1].Value).To(Equal("us-west-2b"))
	})

})
