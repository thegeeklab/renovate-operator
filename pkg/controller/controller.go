package controller

import ctrl "sigs.k8s.io/controller-runtime"

func HandleReconcileResult(res *ctrl.Result, err error) (ctrl.Result, error) {
	if err != nil {
		if res != nil {
			return *res, err
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
