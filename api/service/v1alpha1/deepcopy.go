package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *Service) DeepCopyInto(out *Service) {
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
	out.Spec = ServiceSpec{
		Workspace:         in.Spec.Workspace,
		PreviewEnvironment:   in.Spec.PreviewEnvironment,
		Source:            in.Spec.Source,
		ContainerRegistry: in.Spec.ContainerRegistry,
		Dependencies:      in.Spec.Dependencies,
	}
	out.Status = ServiceStatus{
		State:             in.Status.State,
		Message:           in.Status.Message,
		HealthStatus:      in.Status.HealthStatus,
		PreviewURLs:       in.Status.PreviewURLs,   
		ObservedGeneration: in.Status.ObservedGeneration,
		
	}
	
}


// DeepCopyObject returns a generically typed copy of an object
func (in *Service) DeepCopyObject() runtime.Object {
	out := Service{}
	in.DeepCopyInto(&out)

	return &out
}

// DeepCopyObject returns a generically typed copy of an object
func (in *ServiceList) DeepCopyObject() runtime.Object {
	out := ServiceList{}
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta

	if in.Items != nil {
		out.Items = make([]Service, len(in.Items)) 
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}

	return &out
}
