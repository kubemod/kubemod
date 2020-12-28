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
	"io/ioutil"
	"path"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func loadTestFiles(file1 string, file2 string, expectedDiffFile string) ([]byte, []byte, []byte, error) {
	// Load all files
	file1Contents, err := ioutil.ReadFile(path.Join("testdata/diff/input/", file1))

	if err != nil {
		return nil, nil, nil, err
	}

	file2Contents, err := ioutil.ReadFile(path.Join("testdata/diff/input/", file2))

	if err != nil {
		return nil, nil, nil, err
	}

	expectedDiffContents, err := ioutil.ReadFile(path.Join("testdata/diff/expectations", expectedDiffFile))

	if err != nil {
		return nil, nil, nil, err
	}

	return file1Contents, file2Contents, expectedDiffContents, nil
}

var _ = Describe("package util", func() {

	diffTableFunction := func(file1 string, file2 string, expectedDiffFile string) {
		// Load all files
		file1Contents, file2Contents, expectedDiffContents, err := loadTestFiles(file1, file2, expectedDiffFile)
		Expect(err).NotTo(HaveOccurred())

		diff, err := Diff(file1Contents, file2Contents)
		Expect(err).NotTo(HaveOccurred())

		Expect(string(diff)).To(Equal(string(expectedDiffContents)))
	}

	DescribeTable("Diff", diffTableFunction,
		Entry("Diff should work as expected", "empty.txt", "empty.txt", "empty.txt"),
		Entry("Diff should work as expected", "f1.yaml", "f1.yaml", "empty.txt"),
		Entry("Diff should work as expected", "f1.yaml", "f2.yaml", "f1-f2.txt"),
	)

	Describe("Diff", func() {

		It("should be safe when executed concurrently", func() {

			var wg sync.WaitGroup
			file1Contents, file2Contents, expectedDiffContents, err := loadTestFiles("f1.yaml", "f2.yaml", "f1-f2.txt")
			Expect(err).NotTo(HaveOccurred())

			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer GinkgoRecover()

					diff, err := Diff(file1Contents, file2Contents)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(diff)).To(Equal(string(expectedDiffContents)))
				}()
			}

			wg.Wait()
		})
	})
})
