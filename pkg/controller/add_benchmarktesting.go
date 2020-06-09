package controller

import (
	"github.com/benchmark-testing/benchmarktesting-operator/pkg/controller/benchmarktesting"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, benchmarktesting.Add)
}
