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
	"github.com/kubemod/kubemod/controllers"
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
	RunOperator bool
	RunWebApp   bool

	WebAppAddr               string
	OperatorMetricsAddr      string
	OperatorHealthProbeAddr  string
	ClusterModRulesNamespace string

	EnableLeaderElection bool
	EnableDevModeLog     bool
}

func main() {

	config := &Config{}

	flag.BoolVar(&config.RunOperator, "operator", false, "Run KubeMod operator.")
	flag.BoolVar(&config.RunWebApp, "webapp", false, "Run KubeMod web application.")

	flag.StringVar(&config.WebAppAddr, "webapp-addr", ":8081", "The address the web app binds to.")
	flag.StringVar(&config.OperatorMetricsAddr, "operator-metrics-addr", ":8082", "The address the operator metric endpoint binds to.")
	flag.StringVar(&config.OperatorHealthProbeAddr, "operator-health-probe-addr", ":8083", "The address the operator health endpoints binds to.")
	flag.StringVar(&config.ClusterModRulesNamespace, "cluster-modrules-namespace", "kubemod-system", "The namespace where cluster-wide ModRules are deployed.")
	flag.BoolVar(&config.EnableLeaderElection, "enable-leader-election", false,
		"Enable leader election for KubeMod operator. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&config.EnableDevModeLog, "enable-dev-mode-log", false, "Enable development level logging.")

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

	log := app.NewLogger(app.EnableDevModeLog(config.EnableDevModeLog))

	setupLog := log.WithName("main-setup")

	// Start operator application.
	wg.Add(1)

	go func() {
		defer wg.Done()

		if config.RunOperator {
			_, err := app.InitializeKubeModOperatorApp(
				scheme,
				app.OperatorMetricsAddr(config.OperatorMetricsAddr),
				app.OperatorHealthProbeAddr(config.OperatorHealthProbeAddr),
				controllers.ClusterModRulesNamespace(config.ClusterModRulesNamespace),
				app.EnableLeaderElection(config.EnableLeaderElection),
				log)

			if err != nil {
				errChan <- err
			}
		} else {
			setupLog.Info("skipping operator: -operator option is not set")
		}
	}()

	// Start web application.
	wg.Add(1)

	go func() {
		defer wg.Done()

		if config.RunWebApp {
			_, err := app.InitializeKubeModWebApp(
				config.WebAppAddr,
				app.EnableDevModeLog(config.EnableDevModeLog),
				log)

			if err != nil {
				errChan <- err
			}
		} else {
			setupLog.Info("skipping web app: -webapp option is not set")
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
