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
	// https://github.com/kubernetes-sigs/kind/releases/tag/v0.14.0
	KindNodeImageLatest = KindNodeImageV24
	KindNodeImageV24    = "kindest/node:v1.24.0@sha256:0866296e693efe1fed79d5e6c7af8df71fc73ae45e3679af05342239cdc5bc8e"
	KindNodeImageV23    = "kindest/node:v1.23.6@sha256:b1fa224cc6c7ff32455e0b1fd9cbfd3d3bc87ecaa8fcb06961ed1afb3db0f9ae"
	KindNodeImageV22    = "kindest/node:v1.22.9@sha256:8135260b959dfe320206eb36b3aeda9cffcb262f4b44cda6b33f7bb73f453105"
	KindNodeImageV21    = "kindest/node:v1.21.12@sha256:f316b33dd88f8196379f38feb80545ef3ed44d9197dca1bfd48bcb1583210207"
	KindNodeImageV20    = "kindest/node:v1.20.15@sha256:6f2d011dffe182bad80b85f6c00e8ca9d86b5b8922cdf433d53575c4c5212248"
	KindNodeImageV19    = "kindest/node:v1.19.16@sha256:d9c819e8668de8d5030708e484a9fdff44d95ec4675d136ef0a0a584e587f65c"
	KindNodeImageV18    = "kindest/node:v1.18.20@sha256:738cdc23ed4be6cc0b7ea277a2ebcc454c8373d7d8fb991a7fcdbd126188e6d7"
)

// KindConfigNodeParams is a bare-bones representation of various parameters which can be set to configure a kind node within the `KindConfigTemplate`.
// It is eventually passed into `KindConfigTemplate` through `KindConfigTemplateParams`.
type KindConfigNodeParams struct {
	Role  string `json:"role"`
	Image string `json:"image"`
}

// KindConfigTemplateParams is used to set values within the `KindConfigTemplate`.
type KindConfigTemplateParams struct {
	Name  string                 `json:"name"`
	NoCNI bool                   `json:"noCNI"`
	Nodes []KindConfigNodeParams `json:"nodes"`
}

// NewKindConfig uses given parameters to render a kind configuration (kind.x-k8s.io/v1alpha4).
// name will set the cluster's name.
// nWorkers determines how many worker nodes will be created.
// nControlPlanes determines how many control plane nodes will be created.
// withCNI toggles kind setting up its default CNI upon cluster creation.
// image determines which container image will be used for nodes.
func NewKindConfig(
	logger *log.Logger,
	name string,
	nWorkers int,
	nControlPlanes int,
	withCNI bool,
	image string,
) (string, error) {
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
