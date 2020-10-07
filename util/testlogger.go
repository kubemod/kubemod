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
	"github.com/go-logr/logr"
)

// TLogger is an interface which implements Logf.
type TLogger interface {
	Logf(format string, args ...interface{})
}

// TestLogger is a logr.Logger that prints through a testing.T object.
// Only error logs will have any effect.
type TestLogger struct {
	TLogger TLogger
}

// Info does nothing.
func (TestLogger) Info(_ string, _ ...interface{}) {
	// Do nothing.
}

// Enabled returns false.
func (TestLogger) Enabled() bool {
	return false
}

// Error prints an error.
func (log TestLogger) Error(err error, msg string, args ...interface{}) {
	log.TLogger.Logf("%s: %v -- %v", msg, err, args)
}

// V returns this log.
func (log TestLogger) V(v int) logr.InfoLogger {
	return log
}

// WithName returns this log.
func (log TestLogger) WithName(_ string) logr.Logger {
	return log
}

// WithValues returns this log.
func (log TestLogger) WithValues(_ ...interface{}) logr.Logger {
	return log
}
