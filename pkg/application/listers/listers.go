package listers

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type ServiceLister interface {
	List(selector labels.Selector) ([]runtime.Object, error)
	GetByKey(key string) (runtime.Object, error)
	Service(namespace string) ServiceNamespaceLister
}

type serviceLister struct {
	indexer cache.Indexer
}

func NewServiceLister(indexer cache.Indexer) ServiceLister {
	return &serviceLister{indexer: indexer}
}

func (l *serviceLister) List(selector labels.Selector) ([]runtime.Object, error) {
	var ret []runtime.Object
	err := cache.ListAll(l.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (l *serviceLister) GetByKey(key string) (runtime.Object, error) {
	obj, exists, err := l.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("service resource not found")
	}
	return obj.(runtime.Object), nil
}

func (l *serviceLister) Service(namespace string) ServiceNamespaceLister {
	return &serviceNamespaceLister{
		indexer:   l.indexer,
		namespace: namespace,
	}
}

type ServiceNamespaceLister interface {
	List(selector labels.Selector) ([]runtime.Object, error)
	Get(name string) (runtime.Object, error)
}

type serviceNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

func (l *serviceNamespaceLister) List(selector labels.Selector) ([]runtime.Object, error) {
	var ret []runtime.Object
	err := cache.ListAllByNamespace(l.indexer, l.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (l *serviceNamespaceLister) Get(name string) (runtime.Object, error) {
	key := l.namespace + "/" + name
	obj, exists, err := l.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("service resource not found")
	}
	return obj.(runtime.Object), nil
}
