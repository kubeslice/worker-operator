package manifest

import (
	// corev1 "k8s.io/api/core/v1"
	"io/ioutil"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
)

var log = logger.NewLogger()

type Manifest struct {
	Path string
}

func NewManifest(path string) *Manifest {
	return &Manifest{
		Path: path,
	}

}

func (m *Manifest) ParseDeployment() (*appsv1.Deployment, error) {
	d := &appsv1.Deployment{}

	yamlFile, err := ioutil.ReadFile(m.Path)
	if err != nil {
		log.Error(err, "unable to read yaml file")
		return nil, err
	}

	err = yaml.Unmarshal(yamlFile, d)
	if err != nil {
		log.Error(err, "unable to parse yaml file as Deployment")
		return nil, err
	}

	return d, nil

}
