//go:build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Discovery) DeepCopyInto(out *Discovery) {
	*out = *in
	if in.Suspend != nil {
		in, out := &in.Suspend, &out.Suspend
		*out = new(bool)
		**out = **in
	}
	if in.Filter != nil {
		in, out := &in.Filter, &out.Filter
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Discovery.
func (in *Discovery) DeepCopy() *Discovery {
	if in == nil {
		return nil
	}
	out := new(Discovery)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Logging) DeepCopyInto(out *Logging) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Logging.
func (in *Logging) DeepCopy() *Logging {
	if in == nil {
		return nil
	}
	out := new(Logging)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Platform) DeepCopyInto(out *Platform) {
	*out = *in
	in.Token.DeepCopyInto(&out.Token)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Platform.
func (in *Platform) DeepCopy() *Platform {
	if in == nil {
		return nil
	}
	out := new(Platform)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Renovate) DeepCopyInto(out *Renovate) {
	*out = *in
	in.Platform.DeepCopyInto(&out.Platform)
	if in.DryRun != nil {
		in, out := &in.DryRun, &out.DryRun
		*out = new(bool)
		**out = **in
	}
	if in.Onboarding != nil {
		in, out := &in.Onboarding, &out.Onboarding
		*out = new(bool)
		**out = **in
	}
	if in.AddLabels != nil {
		in, out := &in.AddLabels, &out.AddLabels
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.GithubTokenSelector != nil {
		in, out := &in.GithubTokenSelector, &out.GithubTokenSelector
		*out = new(v1.EnvVarSource)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Renovate.
func (in *Renovate) DeepCopy() *Renovate {
	if in == nil {
		return nil
	}
	out := new(Renovate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Renovator) DeepCopyInto(out *Renovator) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Renovator.
func (in *Renovator) DeepCopy() *Renovator {
	if in == nil {
		return nil
	}
	out := new(Renovator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Renovator) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RenovatorList) DeepCopyInto(out *RenovatorList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Renovator, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RenovatorList.
func (in *RenovatorList) DeepCopy() *RenovatorList {
	if in == nil {
		return nil
	}
	out := new(RenovatorList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RenovatorList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RenovatorSpec) DeepCopyInto(out *RenovatorSpec) {
	*out = *in
	in.Renovate.DeepCopyInto(&out.Renovate)
	in.Discovery.DeepCopyInto(&out.Discovery)
	if in.Suspend != nil {
		in, out := &in.Suspend, &out.Suspend
		*out = new(bool)
		**out = **in
	}
	out.Logging = in.Logging
	out.Worker = in.Worker
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RenovatorSpec.
func (in *RenovatorSpec) DeepCopy() *RenovatorSpec {
	if in == nil {
		return nil
	}
	out := new(RenovatorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RenovatorStatus) DeepCopyInto(out *RenovatorStatus) {
	*out = *in
	in.Phase.DeepCopyInto(&out.Phase)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Repositories != nil {
		in, out := &in.Repositories, &out.Repositories
		*out = make([]Repository, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RenovatorStatus.
func (in *RenovatorStatus) DeepCopy() *RenovatorStatus {
	if in == nil {
		return nil
	}
	out := new(RenovatorStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Repositories) DeepCopyInto(out *Repositories) {
	*out = *in
	if in.Repositories != nil {
		in, out := &in.Repositories, &out.Repositories
		*out = make([]Repository, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Repositories.
func (in *Repositories) DeepCopy() *Repositories {
	if in == nil {
		return nil
	}
	out := new(Repositories)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Repository) DeepCopyInto(out *Repository) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Repository.
func (in *Repository) DeepCopy() *Repository {
	if in == nil {
		return nil
	}
	out := new(Repository)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Worker) DeepCopyInto(out *Worker) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Worker.
func (in *Worker) DeepCopy() *Worker {
	if in == nil {
		return nil
	}
	out := new(Worker)
	in.DeepCopyInto(out)
	return out
}
