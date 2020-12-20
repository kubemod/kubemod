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

package app

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alron/ginlogr"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
)

// KubeModWebApp is the DI container of kubemod web application state.
type KubeModWebApp struct {
}

// NewKubeModWebApp instantiates a kubemod web application.
func NewKubeModWebApp(
	webAppAddr string,
	enableDevModeLog EnableDevModeLog,
	log logr.Logger,
) (*KubeModWebApp, error) {

	setupLog := log.WithName("web-app-setup")
	setupLog.Info("web app server is starting to listen", "addr", webAppAddr)

	if enableDevModeLog {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	r.Use(ginlogr.RecoveryWithLogr(log, time.RFC3339, false, true))

	// Set up the API routes.
	setupRoutes(r)

	// Run the server - this will block until the process is terminated through a SIGTERM or SIGINT.
	run(r, webAppAddr, log)

	return &KubeModWebApp{}, nil
}

// run starts a web server with the given router as a handler and blocks until the process is terminated.
func run(router *gin.Engine, webAppAddr string, log logr.Logger) {

	srv := &http.Server{
		Addr:    webAppAddr,
		Handler: router,
	}

	// Initializing the server in a goroutine so that it won't block the graceful shutdown handling below.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, "KubeMod web app failed to listen and serve", "addr", webAppAddr)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	quit := make(chan os.Signal)

	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Info("shutting down KubeMod web app")

	// The context is used to inform the server it has 10 seconds to finish the request it is currently handling.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error(err, "KubeMod web app forced to shutdown")
	}

	log.Info("KubeMod web app exited")
}
