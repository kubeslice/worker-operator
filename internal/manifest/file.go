package manifest

import (
	// corev1 "k8s.io/api/core/v1"
	"io/ioutil"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
	"encoding/json"
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

func (m *Manifest) Parse(v interface{}) error {
	jsonFile, err := ioutil.ReadFile(m.Path)
	if err != nil {
		log.Error(err, "unable to read json file")
		return err
	}

	err = json.Unmarshal(jsonFile, v)
	if err != nil {
		log.Error(err, "unable to parse json file as Deployment")
		return err
	}

	return nil

}
