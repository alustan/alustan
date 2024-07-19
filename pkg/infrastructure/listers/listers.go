package listers

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type TerraformLister interface {
	List(selector labels.Selector) ([]runtime.Object, error)
	GetByKey(key string) (runtime.Object, error)
	Terraform(namespace string) TerraformNamespaceLister
}

type terraformLister struct {
	indexer cache.Indexer
}

func NewTerraformLister(indexer cache.Indexer) TerraformLister {
	return &terraformLister{indexer: indexer}
}

func (l *terraformLister) List(selector labels.Selector) ([]runtime.Object, error) {
	var ret []runtime.Object
	err := cache.ListAll(l.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (l *terraformLister) GetByKey(key string) (runtime.Object, error) {
	obj, exists, err := l.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("terraform resource not found")
	}
	return obj.(runtime.Object), nil
}

func (l *terraformLister) Terraform(namespace string) TerraformNamespaceLister {
	return &terraformNamespaceLister{
		indexer:   l.indexer,
		namespace: namespace,
	}
}

type TerraformNamespaceLister interface {
	List(selector labels.Selector) ([]runtime.Object, error)
	Get(name string) (runtime.Object, error)
}

type terraformNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

func (l *terraformNamespaceLister) List(selector labels.Selector) ([]runtime.Object, error) {
	var ret []runtime.Object
	err := cache.ListAllByNamespace(l.indexer, l.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (l *terraformNamespaceLister) Get(name string) (runtime.Object, error) {
	key := l.namespace + "/" + name
	obj, exists, err := l.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("terraform resource not found")
	}
	return obj.(runtime.Object), nil
}
