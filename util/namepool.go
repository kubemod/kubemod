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
	"sync/atomic"

	"github.com/segmentio/ksuid"
)

// NamePool - represents a pool of randomly generated strings.
type NamePool struct {
	pool             []string
	size             int
	incrementalIndex uint64
}

// NewNamePool - return a newly instantiated NamePool
func NewNamePool(size int) *NamePool {
	pool := make([]string, size)

	for i := 0; i < size; i++ {
		pool[i] = ksuid.New().String()
	}

	return &NamePool{
		pool:             pool,
		size:             size,
		incrementalIndex: 0,
	}
}

// GetNextName returns the next randomly generated name.
func (np *NamePool) GetNextName() string {
	var index = atomic.AddUint64(&np.incrementalIndex, 1)
	return np.pool[index%uint64(np.size)]
}

// ResetIndex resets the internal incremental index.
func (np *NamePool) ResetIndex() {
	np.incrementalIndex = 0
}
