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
	"sync/atomic"
	"testing"

	"github.com/kubemod/kubemod/api/v1beta1"
	"github.com/kubemod/kubemod/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BenchmarkModRuleStorePutParallel(b *testing.B) {
	var counter uint64
	namespacePool := util.NewNamePool(5)
	namePool := util.NewNamePool(50)

	testBed := InitializeModRuleStoreTestBed(b)

	rs := testBed.modRuleStore

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atomic.AddUint64(&counter, 1)

			err := rs.Put(&v1beta1.ModRule{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespacePool.GetNextName(),
					Name:      namePool.GetNextName(),
				},
				Spec: v1beta1.ModRuleSpec{
					Match: []v1beta1.MatchItem{
						v1beta1.MatchItem{
							Query: `$.kind == "Pod"`,
						},
						v1beta1.MatchItem{
							Query: `$.metadata.labels.app =~ "nginx"`,
						},
					},
				},
			})

			if err != nil {
				b.Error(err)
			}
		}
	})
}
