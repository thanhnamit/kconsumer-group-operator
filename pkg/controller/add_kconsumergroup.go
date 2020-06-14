package controller

import (
	"github.com/thanhnamit/kconsumer-group-operator/pkg/controller/kconsumergroup"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, kconsumergroup.Add)
}
