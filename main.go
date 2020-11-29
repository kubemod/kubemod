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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	apiv1beta1 "github.com/kubemod/kubemod/api/v1beta1"
	"github.com/kubemod/kubemod/app"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = apiv1beta1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

// Config holds the application configuration settings.
type Config struct {
	MetricsAddr          string
	EnableLeaderElection bool
	EnableDevModeLog     bool
}

func main() {

	config := &Config{}

	flag.StringVar(&config.MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&config.EnableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&config.EnableDevModeLog, "enable-dev-mode-log", false,
		"Enable development level logging.")

	flag.Parse()

	err := run(config)

	if err != nil {
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder
}

func run(config *Config) error {
	var wg sync.WaitGroup

	errChan := make(chan error)
	errors := []string{}
	errorPumpDone := make(chan bool)

	// Start operator application.
	wg.Add(1)

	logger := app.NewLogger(app.EnableDevModeLog(config.EnableDevModeLog))

	go func() {
		defer wg.Done()

		_, err := app.InitializeKubeModOperatorApp(
			scheme,
			config.MetricsAddr,
			app.EnableLeaderElection(config.EnableLeaderElection),
			logger)

		if err != nil {
			errChan <- err
		}
	}()

	// Start error channel pump.
	go func(done chan<- bool) {
		for err := range errChan {
			errors = append(errors, err.Error())
		}

		done <- true
	}(errorPumpDone)

	// Wait for the applications to complete.
	wg.Wait()

	// Tell the error pump we're done.
	close(errChan)
	// Wait for the error pump to complete.
	<-errorPumpDone

	// If there are errors reported by the servers, aggregate them into a single error.
	if len(errors) > 0 {
		err := fmt.Errorf("%v", strings.Join(errors, ";"))
		return err
	}

	return nil
}
