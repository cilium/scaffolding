package toolkit

import (
	"encoding/json"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
)

const (
	KindConfigTemplate = `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: {{ .Name }}
networking:
  disableDefaultCNI: {{ .NoCNI }}
nodes:
{{- range .Nodes }}
- role: {{ .Role }}
  image: {{ .Image }}
{{- end }}
`
)

type KindConfigNodeParams struct {
	Role  string `json:"role"`
	Image string `json:"image"`
}

type KindConfigTemplateParams struct {
	Name  string                 `json:"name"`
	NoCNI bool                   `json:"noCNI"`
	Nodes []KindConfigNodeParams `json:"nodes"`
}

func NewKindConfig(logger *log.Logger, name string, nWorkers int, nControlPlanes int, withCNI bool, image string) (string, error) {
	kindConfigTemplateParams := KindConfigTemplateParams{
		Name:  name,
		NoCNI: !withCNI,
		Nodes: make([]KindConfigNodeParams, 0),
	}
	n := 0
	for n < nWorkers {
		kindConfigTemplateParams.Nodes = append(
			kindConfigTemplateParams.Nodes,
			KindConfigNodeParams{
				Role:  "worker",
				Image: image,
			},
		)
		n++
	}
	n = 0
	for n < nControlPlanes {
		kindConfigTemplateParams.Nodes = append(
			kindConfigTemplateParams.Nodes,
			KindConfigNodeParams{
				Role:  "control-plane",
				Image: image,
			},
		)
		n++
	}

	templateStringBuilder := &strings.Builder{}
	configTemplateName := "kindConfig-" + name + "-" + RandomString(5)
	template := template.New(configTemplateName)

	logger.WithField("template", KindConfigTemplate).Debug("parsing kind config template")
	_, err := template.Parse(KindConfigTemplate)
	if err != nil {
		logger.WithError(err).Error("unable to parse kind config template")
		return "", err
	}

	marshalledParams, err := json.Marshal(kindConfigTemplateParams)
	if err != nil {
		logger.WithError(err).
			WithField("params", kindConfigTemplateParams).
			Error("unable to unmarshal template params")
	}
	logger.WithField("params", string(marshalledParams)).Debug("rendering template with params")
	err = template.Execute(templateStringBuilder, kindConfigTemplateParams)
	if err != nil {
		logger.WithError(err).Error("unable to render kind config template")
		return "", err
	}

	return templateStringBuilder.String(), nil
}
