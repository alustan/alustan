package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *App) DeepCopyInto(out *App) {
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
	out.Spec = AppSpec{
		Environment:         in.Spec.Environment,
		PreviewEnvironment:   in.Spec.PreviewEnvironment,
		Source:            in.Spec.Source,
		ContainerRegistry: in.Spec.ContainerRegistry,
		Dependencies:      in.Spec.Dependencies,
	}
	out.Status = AppStatus{
		State:             in.Status.State,
		Message:           in.Status.Message,
		HealthStatus:      in.Status.HealthStatus,
		PreviewURLs:       in.Status.PreviewURLs,   
		ObservedGeneration: in.Status.ObservedGeneration,
		
	}
	
}


// DeepCopyObject returns a generically typed copy of an object
func (in *App) DeepCopyObject() runtime.Object {
	out := App{}
	in.DeepCopyInto(&out)

	return &out
}

// DeepCopyObject returns a generically typed copy of an object
func (in *AppList) DeepCopyObject() runtime.Object {
	out := AppList{}
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta

	if in.Items != nil {
		out.Items = make([]App, len(in.Items)) 
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}

	return &out
}
