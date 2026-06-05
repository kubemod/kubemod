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

	"github.com/golang/mock/gomock"
	"github.com/kubemod/kubemod/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("PodBindingWebhookHandler", func() {
	var (
		mockCtrl   *gomock.Controller
		mockClient *mocks.MockClient
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClient(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	It("should make no API calls and allow the request when node refs are disabled", func() {
		handler := &PodBindingWebhookHandler{
			client:              mockClient,
			log:                 NewTestLogger(GinkgoT()),
			syntheticRefsConfig: SyntheticRefsConfig{DisableNodeRefs: true},
		}

		request := admission.Request{
			AdmissionRequest: admissionv1beta1.AdmissionRequest{
				Resource:    metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
				SubResource: "binding",
				Operation:   "CREATE",
			},
		}

		// No mockClient.EXPECT() calls are set: any Get/Update would be an unexpected call and fail the test.
		response := handler.Handle(context.Background(), request)
		Expect(response.Allowed).To(BeTrue())
	})
})
