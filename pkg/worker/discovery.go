package worker

import (
	"context"
	"strings"

	"github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/discovery"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/renovate"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (w *Worker) reconcileDiscovery(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	// Cronjob
	jobSpec := w.createDiscoveryJobSpec()

	expectedCronJob, err := w.createDiscoveryCronJob(jobSpec)
	if err != nil {
		return &ctrl.Result{}, err
	}

	ctxLogger = ctxLogger.WithValues("cronJob", expectedCronJob.Name)
	key := types.NamespacedName{
		Namespace: expectedCronJob.Namespace,
		Name:      expectedCronJob.Name,
	}

	currentCronJob := batchv1.CronJob{}

	err = w.client.Get(ctx, key, &currentCronJob)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = w.client.Create(ctx, expectedCronJob); err != nil {
				ctxLogger.Error(err, "Failed to create Cronjob")

				return &ctrl.Result{}, err
			}

			ctxLogger.Info("Created Cronjob")

			return &ctrl.Result{Requeue: true}, nil
		}

		return &ctrl.Result{}, err
	}

	if !equality.Semantic.DeepEqual(*expectedCronJob, currentCronJob) {
		ctxLogger.Info("Updating CronJob")

		err := w.client.Update(ctx, expectedCronJob)
		if err != nil {
			ctxLogger.Error(err, "Failed to update CronJob")

			return &ctrl.Result{}, err
		}

		ctxLogger.Info("Updated CronJob")

		return &ctrl.Result{Requeue: true}, nil
	}

	return &ctrl.Result{}, nil
}

func (w *Worker) reconcileServiceAccount(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	// Create ServiceAccount
	expectedSA, err := w.createServiceAccount()
	if err != nil {
		return &ctrl.Result{}, err
	}

	currentSA := &corev1.ServiceAccount{}

	err = w.client.Get(ctx, w.req.NamespacedName, currentSA)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = w.client.Create(ctx, expectedSA); err != nil {
				ctxLogger.Error(err, "Failed to create ServiceAccount")

				return nil, err
			}

			ctxLogger.Info("Created ServiceAccount")

			return &ctrl.Result{Requeue: true}, nil
		}

		return nil, err
	}

	// Create Role
	expectedRole, expectedRoleBinding, err := w.createRBAC()
	if err != nil {
		return &ctrl.Result{}, err
	}

	currentRole := &rbacv1.Role{}

	err = w.client.Get(ctx, w.req.NamespacedName, currentRole)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = w.client.Create(ctx, expectedRole); err != nil {
				ctxLogger.Error(err, "Failed to create Role")

				return nil, err
			}

			ctxLogger.Info("Created Role")

			return &ctrl.Result{Requeue: true}, nil
		}

		return nil, err
	}

	currentRoleBinding := &rbacv1.RoleBinding{}

	err = w.client.Get(ctx, w.req.NamespacedName, currentRoleBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			if err = w.client.Create(ctx, expectedRoleBinding); err != nil {
				ctxLogger.Error(err, "Failed to create RoleBinding")

				return nil, err
			}

			ctxLogger.Info("Created RoleBinding")

			return &ctrl.Result{Requeue: true}, nil
		}

		return nil, err
	}

	return &ctrl.Result{}, nil
}

func (w *Worker) createServiceAccount() (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metadata.GenericMetaData(w.req),
	}

	if err := controllerutil.SetControllerReference(w.instance, sa, w.scheme); err != nil {
		return nil, err
	}

	return sa, nil
}

func (w *Worker) createRBAC() (*rbacv1.Role, *rbacv1.RoleBinding, error) {
	role := &rbacv1.Role{
		ObjectMeta: metadata.GenericMetaData(w.req),
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{v1beta1.GroupVersion.Group},
				Resources: []string{"renovators/status"},
				Verbs:     []string{"get", "update", "patch"},
			},
		},
	}
	if err := controllerutil.SetControllerReference(w.instance, role, w.scheme); err != nil {
		return nil, nil, err
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metadata.GenericMetaData(w.req),
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      metadata.GenericMetaData(w.req).Name,
				Namespace: w.req.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     metadata.GenericMetaData(w.req).Name,
		},
	}
	if err := controllerutil.SetControllerReference(w.instance, roleBinding, w.scheme); err != nil {
		return nil, nil, err
	}

	return role, roleBinding, nil
}

func (w *Worker) createDiscoveryCronJob(jobSpec batchv1.JobSpec) (*batchv1.CronJob, error) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metadata.DiscoveryMetaData(w.req),
		Spec: batchv1.CronJobSpec{
			Schedule:          w.instance.Spec.Discovery.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			Suspend:           w.instance.Spec.Suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: jobSpec,
			},
		},
	}

	if err := controllerutil.SetControllerReference(w.instance, cronJob, w.scheme); err != nil {
		return nil, err
	}

	return cronJob, nil
}

func (w *Worker) createDiscoveryJobSpec() batchv1.JobSpec {
	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				ServiceAccountName: metadata.GenericMetaData(w.req).Name,
				RestartPolicy:      corev1.RestartPolicyNever,
				Volumes: renovate.StandardVolumes(corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: w.instance.Name,
						},
					},
				}),
				InitContainers: []corev1.Container{
					renovate.Container(w.instance, []corev1.EnvVar{
						{
							Name:  "RENOVATE_AUTODISCOVER",
							Value: "true",
						},
						{
							Name:  "RENOVATE_AUTODISCOVER_FILTER",
							Value: strings.Join(w.instance.Spec.Discovery.Filter, ","),
						},
					}, []string{"--write-discovered-repos", renovate.FileRenovateConfigOutput}),
				},
				Containers: []corev1.Container{
					{
						Name:            "renovate-discovery",
						Image:           w.instance.Spec.Image,
						ImagePullPolicy: w.instance.Spec.ImagePullPolicy,
						Env: []corev1.EnvVar{
							{
								Name:  discovery.EnvRenovatorInstanceName,
								Value: w.instance.Name,
							},
							{
								Name:  discovery.EnvRenovatorInstanceNamespace,
								Value: w.instance.Namespace,
							},
							{
								Name:  discovery.EnvRenovateOutputFile,
								Value: renovate.FileRenovateConfigOutput,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      renovate.VolumeWorkDir,
								MountPath: renovate.DirRenovateBase,
							},
						},
					},
				},
			},
		},
	}
}
