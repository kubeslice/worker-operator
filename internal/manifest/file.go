package manifest

import (
	// corev1 "k8s.io/api/core/v1"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"encoding/json"

	"bitbucket.org/realtimeai/kubeslice-operator/internal/logger"
)

var log = logger.NewLogger()

func GetManifestPath(file string) string {
	dir := os.Getenv("MANIFEST_PATH")
	if dir != "" {
		return path.Join(dir, file+".json")
	}

	return path.Join("../../files/manifests", file+".json")
}

type Manifest struct {
	Slice string
	Path  string
}

func NewManifest(f string, slice string) *Manifest {
	return &Manifest{
		Path:  GetManifestPath(f),
		Slice: slice,
	}
}

func (m *Manifest) Parse(v interface{}) error {
	jsonFile, err := ioutil.ReadFile(m.Path)
	if err != nil {
		log.Error(err, "unable to read json file")
		return err
	}

	f := strings.ReplaceAll(string(jsonFile), "SLICE", m.Slice)

	err = json.Unmarshal([]byte(f), v)
	if err != nil {
		log.Error(err, "unable to parse json file as Deployment")
		return err
	}

	return nil

}
