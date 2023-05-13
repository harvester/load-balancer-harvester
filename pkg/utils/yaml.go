package utils

import (
	"bufio"
	"bytes"
	"io"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	kubevirtv1 "kubevirt.io/api/core/v1"

	lbv1alpha1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	lbv1beta1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

// Add the object to the scheme first if you want ParseFromFile to be able to decode the object
var localSchemeBuilder = runtime.SchemeBuilder{
	lbv1beta1.AddToScheme,
	lbv1alpha1.AddToScheme,
	kubevirtv1.AddToScheme,
}

// ParseFromFile parses a YAML file into a list of Kubernetes objects
func ParseFromFile(yamlFilePath string) ([]runtime.Object, error) {
	// Read the YAML file into a byte array
	data, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return nil, err
	}

	multidocReader := yaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	objs := make([]runtime.Object, 0)

	decoder := scheme.Codecs.UniversalDeserializer()
	for {
		buf, err := multidocReader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		obj, _, err := decoder.Decode(buf, nil, nil)
		if err != nil {
			return nil, err
		}
		objs = append(objs, obj)
	}

	return objs, nil
}

func init() {
	utilruntime.Must(localSchemeBuilder.AddToScheme(scheme.Scheme))
}
