package outputfile

import (
	"io/ioutil"

	"github.com/daspawnw/prometheus-aws-discovery/pkg/endpoints"
	log "github.com/sirupsen/logrus"
)

type OutputFile struct {
	FilePath string
}

func (o OutputFile) Write(instances endpoints.DiscoveredInstances) error {
	content, err := endpoints.ToJsonString(instances)
	if err != nil {
		log.Error("Failed to convert instances to json string with error", err)
		return err
	}

	e := ioutil.WriteFile(o.FilePath, content, 0666)
	if e != nil {
		log.Errorf("Failed to write to filepath %s", o.FilePath)
	}
	return e
}
