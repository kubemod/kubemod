// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package app

import (
	"github.com/go-logr/logr"
	"github.com/kubemod/kubemod/controllers"
	"github.com/kubemod/kubemod/core"
	"github.com/kubemod/kubemod/expressions"
	"k8s.io/apimachinery/pkg/runtime"
)

// Injectors from wire.go:

func InitializeKubeModOperatorApp(scheme *runtime.Scheme, metricsAddr string, enableLeaderElection EnableLeaderElection, log logr.Logger) (*KubeModOperatorApp, error) {
	manager, err := NewControllerManager(scheme, metricsAddr, enableLeaderElection, log)
	if err != nil {
		return nil, err
	}
	language := expressions.NewJSONPathLanguage()
	modRuleStoreItemFactory := core.NewModRuleStoreItemFactory(language, log)
	modRuleStore := core.NewModRuleStore(modRuleStoreItemFactory, log)
	modRuleReconciler, err := controllers.NewModRuleReconciler(manager, modRuleStore, log)
	if err != nil {
		return nil, err
	}
	dragnetWebhookHandler := core.NewDragnetWebhookHandler(manager, modRuleStore, log)
	kubeModOperatorApp, err := NewKubeModOperatorApp(scheme, manager, modRuleReconciler, dragnetWebhookHandler, log)
	if err != nil {
		return nil, err
	}
	return kubeModOperatorApp, nil
}

func InitializeKubeModWebApp(webAppAddr string, log logr.Logger) (*KubeModWebApp, error) {
	kubeModWebApp, err := NewKubeModWebApp(webAppAddr, log)
	if err != nil {
		return nil, err
	}
	return kubeModWebApp, nil
}

// wire.go:

type EnableLeaderElection bool

type EnableDevModeLog bool
