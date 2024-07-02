package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/alustan/pkg/controller"
	"github.com/alustan/pkg/schematypes"
)

// MockedKubernetesClient is a mocked Kubernetes client for testing purposes
type MockedKubernetesClient struct {
	mock.Mock
}

func (m *MockedKubernetesClient) CoreV1() kubernetes.CoreV1Interface {
	return &MockedCoreV1Interface{}
}

type MockedCoreV1Interface struct {
	mock.Mock
}

func (m *MockedCoreV1Interface) Secrets(namespace string) kubernetes.SecretInterface {
	return &MockedSecretInterface{}
}

type MockedSecretInterface struct {
	mock.Mock
}

func (m *MockedSecretInterface) Create(ctx context.Context, secret *v1.Secret, opts metav1.CreateOptions) (*v1.Secret, error) {
	args := m.Called(ctx, secret, opts)
	return args.Get(0).(*v1.Secret), args.Error(1)
}

func (m *MockedSecretInterface) Update(ctx context.Context, secret *v1.Secret, opts metav1.UpdateOptions) (*v1.Secret, error) {
	args := m.Called(ctx, secret, opts)
	return args.Get(0).(*v1.Secret), args.Error(1)
}

func (m *MockedSecretInterface) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Secret, error) {
	args := m.Called(ctx, name, opts)
	return args.Get(0).(*v1.Secret), args.Error(1)
}

// MockedDynamicClient is a mocked Dynamic Kubernetes client for testing purposes
type MockedDynamicClient struct {
	mock.Mock
}

func (m *MockedDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &MockedNamespaceableResourceInterface{}
}

type MockedNamespaceableResourceInterface struct {
	mock.Mock
}

func (m *MockedNamespaceableResourceInterface) Namespace(namespace string) dynamic.ResourceInterface {
	return &MockedResourceInterface{}
}

type MockedResourceInterface struct {
	mock.Mock
}

func (m *MockedResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(*unstructured.UnstructuredList), args.Error(1)
}

func (m *MockedResourceInterface) Get(ctx context.Context, name string, opts metav1.GetOptions) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, opts)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockedResourceInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, obj, opts)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *MockedResourceInterface) Apply(ctx context.Context, obj *unstructured.Unstructured, opts metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, obj, opts)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

// TestServeHTTP tests the ServeHTTP method of Controller
func TestServeHTTP(t *testing.T) {
	ctrl := setupController()

	// Mock request and response
	syncRequest := schematypes.SyncRequest{
		Parent: schematypes.TerraformConfig{
			Metadata: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "default",
			},
			Spec: schematypes.TerraformConfigSpec{},
		},
	}
	mockGinContext := createMockGinContext(syncRequest)

	// Call ServeHTTP method
	ctrl.ServeHTTP(mockGinContext)

	// Assert response
	assert.Equal(t, http.StatusOK, mockGinContext.Writer.Status())
}

// TestIsCRDChanged tests the IsCRDChanged method of Controller
func TestIsCRDChanged(t *testing.T) {
	ctrl := setupController()

	// Mock cache
	ctrl.Cache()["exampleCRD"] = "oldHash"

	// New spec with different hash
	newSpec := schematypes.TerraformConfigSpec{}
	changed := ctrl.IsCRDChanged(schematypes.SyncRequest{
		Parent: schematypes.TerraformConfig{
			Metadata: metav1.ObjectMeta{Name: "exampleCRD"},
			Spec:     newSpec,
		},
	})
	assert.True(t, changed)

	// Same spec should not be changed
	ctrl.Cache()["exampleCRD"] = controller.HashSpec(newSpec)
	unchanged := ctrl.IsCRDChanged(schematypes.SyncRequest{
		Parent: schematypes.TerraformConfig{
			Metadata: metav1.ObjectMeta{Name: "exampleCRD"},
			Spec:     newSpec,
		},
	})
	assert.False(t, unchanged)
}

// TestUpdateCache tests the UpdateCache method of Controller
func TestUpdateCache(t *testing.T) {
	ctrl := setupController()

	// Update cache
	newSpec := schematypes.TerraformConfigSpec{}
	ctrl.UpdateCache(schematypes.SyncRequest{
		Parent: schematypes.TerraformConfig{
			Metadata: metav1.ObjectMeta{Name: "exampleCRD"},
			Spec:     newSpec,
		},
	})

	// Verify cache updated
	assert.Equal(t, 1, len(ctrl.Cache()))
}

// TestReconcile tests the Reconcile method of Controller
func TestReconcile(t *testing.T) {
	ctrl := setupController()

	// Mock dynamic client response
	mockResourceInterface := new(MockedResourceInterface)
	mockResourceInterface.On("List", mock.Anything, mock.Anything).Return(&unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "example",
						"namespace": "default",
					},
					"spec": map[string]interface{}{},
				},
			},
		},
	}, nil)
	ctrl.DynClient().(*MockedDynamicClient).On("Resource", mock.Anything).Return(mockResourceInterface)

	// Run Reconcile
	go ctrl.Reconcile()

	// Sleep to allow reconcile loop to run
	time.Sleep(2 * time.Second)

	// Verify dynamic client interactions
	mockResourceInterface.AssertCalled(t, "List", mock.Anything, mock.Anything)
}

// helper function to set up a Controller instance with mocked clients
func setupController() *controller.Controller {
	mockClientset := new(MockedKubernetesClient)
	mockDynClient := new(MockedDynamicClient)
	return controller.NewController(mockClientset, mockDynClient, time.Minute)
}

// helper function to create a mock Gin context
func createMockGinContext(obj interface{}) *gin.Context {
	ctx, _ := gin.CreateTestContext(&bytes.Buffer{})
	body, _ := json.Marshal(obj)
	ctx.Request = &http.Request{
		Method: "POST",
		Body:   &mockBody{body: body},
		Header: http.Header{"Content-Type": []string{"application/json"}},
	}
	return ctx
}

// mockBody is a mock implementation of io.ReadCloser
type mockBody struct {
	body []byte
}

func (mb *mockBody) Read(p []byte) (n int, err error) {
	copy(p, mb.body)
	return len(mb.body), nil
}

func (mb *mockBody) Close() error {
	return nil
}
