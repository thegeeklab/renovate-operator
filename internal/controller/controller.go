package controller

import (
	"errors"

	ctrl "sigs.k8s.io/controller-runtime"
)

var ErrNoRenovateConfigFound = errors.New("no RenovateConfig found")

func HandleReconcileResult(res *ctrl.Result, err error) (ctrl.Result, error) {
	if err != nil {
		if res != nil {
			return *res, err
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
