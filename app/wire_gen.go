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

func InitializeKubeModOperatorApp(scheme *runtime.Scheme, metricsAddr string, enableLeaderElection EnableLeaderElection, logger logr.Logger) (*KubeModOperatorApp, error) {
	manager, err := NewControllerManager(scheme, metricsAddr, enableLeaderElection, logger)
	if err != nil {
		return nil, err
	}
	language := expressions.NewJSONPathLanguage()
	modRuleStoreItemFactory := core.NewModRuleStoreItemFactory(language, logger)
	modRuleStore := core.NewModRuleStore(modRuleStoreItemFactory)
	modRuleReconciler, err := controllers.NewModRuleReconciler(manager, modRuleStore, logger)
	if err != nil {
		return nil, err
	}
	dragnetWebhookHandler := core.NewDragnetWebhookHandler(manager, modRuleStore, logger)
	kubeModOperatorApp, err := NewKubeModOperatorApp(scheme, manager, modRuleReconciler, dragnetWebhookHandler, logger)
	if err != nil {
		return nil, err
	}
	return kubeModOperatorApp, nil
}

// wire.go:

type EnableLeaderElection bool

type EnableDevModeLog bool
