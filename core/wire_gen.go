// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package core

import (
	"github.com/go-logr/logr"
	"github.com/kubemod/kubemod/expressions"
	"github.com/kubemod/kubemod/util"
)

// Injectors from wire.go:

func InitializeModRuleStoreTestBed(clusterModRulesNamespace ClusterModRulesNamespace, tLogger util.TLogger) *ModRuleStoreTestBed {
	language := expressions.NewJSONPathLanguage()
	logger := NewTestLogger(tLogger)
	modRuleStoreItemFactory := NewModRuleStoreItemFactory(language, logger)
	modRuleStore := NewModRuleStore(modRuleStoreItemFactory, clusterModRulesNamespace, logger)
	modRuleStoreTestBed := NewModRuleStoreTestBed(modRuleStore)
	return modRuleStoreTestBed
}

func InitializeModRuleStoreItemTestBed(tLogger util.TLogger) *ModRuleStoreItemTestBed {
	language := expressions.NewJSONPathLanguage()
	logger := NewTestLogger(tLogger)
	modRuleStoreItemFactory := NewModRuleStoreItemFactory(language, logger)
	modRuleStoreItemTestBed := NewModRuleStoreItemTestBed(modRuleStoreItemFactory)
	return modRuleStoreItemTestBed
}

// wire.go:

// ModRuleStoreTestBed is a structure used to provide services for ModRuleStore tests.
type ModRuleStoreTestBed struct {
	modRuleStore *ModRuleStore
}

// ModRuleStoreItemTestBed is used in ModRuleStoreItem test.
type ModRuleStoreItemTestBed struct {
	itemFactory *ModRuleStoreItemFactory
}

// NewLogger instantiates a new testing logger.
func NewTestLogger(tLogger util.TLogger) logr.Logger {
	return util.TestLogger{TLogger: tLogger}
}

// NewModRuleStoreTestBed instantiates a new test bed.
func NewModRuleStoreTestBed(modRuleStore *ModRuleStore) *ModRuleStoreTestBed {
	return &ModRuleStoreTestBed{
		modRuleStore: modRuleStore,
	}
}

// NewModRuleStoreItemTestBed instantiates a new test bed.
func NewModRuleStoreItemTestBed(itemFactory *ModRuleStoreItemFactory) *ModRuleStoreItemTestBed {
	return &ModRuleStoreItemTestBed{
		itemFactory: itemFactory,
	}
}
