package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *Terraform) DeepCopyInto(out *Terraform) {
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
	out.Spec = TerraformSpec{
		Variables:         in.Spec.Variables,
		Scripts:           in.Spec.Scripts,
		PostDeploy:        in.Spec.PostDeploy,
		ContainerRegistry: in.Spec.ContainerRegistry,
	}
	out.Status = TerraformStatus{
		State:             in.Status.State,
		Message:           in.Status.Message,
		Output:            in.Status.Output,
		IngressURLs:       in.Status.IngressURLs,
		Credentials:       in.Status.Credentials,
		PostDeployOutput:   in.Status.PostDeployOutput,
		ObservedGeneration: in.Status.ObservedGeneration,
		
	}
	
}


// DeepCopyObject returns a generically typed copy of an object
func (in *Terraform) DeepCopyObject() runtime.Object {
	out := Terraform{}
	in.DeepCopyInto(&out)

	return &out
}

// DeepCopyObject returns a generically typed copy of an object
func (in *TerraformList) DeepCopyObject() runtime.Object {
	out := TerraformList{}
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta

	if in.Items != nil {
		out.Items = make([]Terraform, len(in.Items)) 
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}

	return &out
}
