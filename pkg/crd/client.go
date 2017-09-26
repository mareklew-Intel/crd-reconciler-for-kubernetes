package crd

import (
	"fmt"
	"time"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const apiRoot = "/api"

// NewClient returns a new REST client for the supplied CRD handle.
func NewClient(config rest.Config, h *Handle) (*rest.RESTClient, error) {
	scheme := runtime.NewScheme()

	scheme.AddKnownTypes(h.SchemaGroupVersion, h.ResourceType, h.ResourceListType)
	metav1.AddToGroupVersion(scheme, h.SchemaGroupVersion)

	config.GroupVersion = &h.SchemaGroupVersion
	config.APIPath = apiRoot
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// WriteDefinition creates the supplied CRD to the Kubernetes API server
// using the supplied client set.
func WriteDefinition(clientset apiextensionsclient.Interface, h *Handle) error {
	_, err := clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(h.Definition)
	if err != nil {
		return err
	}

	var crd *apiextensionsv1beta1.CustomResourceDefinition
	// Wait for CRD to be established.
	err = wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		crd, err = clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Get(h.resourceName(), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1beta1.Established:
				if cond.Status == apiextensionsv1beta1.ConditionTrue {
					return true, err
				}
			case apiextensionsv1beta1.NamesAccepted:
				if cond.Status == apiextensionsv1beta1.ConditionFalse {
					fmt.Printf("Name conflict: %v\n", cond.Reason)
				}
			}
		}
		return false, err
	})
	if err != nil {
		deleteErr := DeleteDefinition(clientset, h)
		if deleteErr != nil {
			return errors.NewAggregate([]error{err, deleteErr})
		}
		return err
	}

	// Update the definition in the supplied handle.
	h.Definition = crd

	return nil
}

// DeleteDefinition removes the supplied CRD to the Kubernetes API server
// using the supplied client set.
func DeleteDefinition(clientset apiextensionsclient.Interface, h *Handle) error {
	return clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(h.Definition.Name, nil)
}
