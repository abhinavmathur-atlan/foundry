package ecsec2terraformcasting

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"

	"github.com/signoz/foundry/api/v1alpha1/infrastructure"
	"github.com/signoz/foundry/internal/domain"
	foundryerrors "github.com/signoz/foundry/internal/errors"
	infrastructuremolding "github.com/signoz/foundry/internal/molding/infrastructure"
	"github.com/signoz/foundry/internal/molding/infrastructure/resourcemolding"
	"github.com/signoz/foundry/internal/pourer"
)

// cloudInitData is what a node needs to know at first boot: which cluster to
// join, what to advertise itself as, and whether it has a disk to mount.
type cloudInitData struct {
	Cluster    string
	Selector   map[string]string
	DataVolume bool
}

type ecsEc2TerraformCasting struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *ecsEc2TerraformCasting {
	return &ecsEc2TerraformCasting{logger: logger}
}

func (c *ecsEc2TerraformCasting) Enricher(ctx context.Context, config *infrastructure.Casting) (infrastructuremolding.MoldingEnricher, error) {
	return newEcsEc2TerraformMoldingEnricher(), nil
}

func (c *ecsEc2TerraformCasting) Forge(ctx context.Context, config infrastructure.Casting, p *pourer.Pourer) error {
	doc := config.Spec.Resource.Status.Config.Data[resourcemolding.ResourceConfigName]

	if doc == "" {
		return foundryerrors.Newf(foundryerrors.TypeInternal, "resource config %q is missing from the resource status", resourcemolding.ResourceConfigName)
	}

	resource := &infrastructure.ResourceConfig{}
	if err := domain.UnmarshalYAML([]byte(doc), resource); err != nil {
		return foundryerrors.Wrapf(err, foundryerrors.TypeInternal, "failed to unmarshal resource config")
	}

	// The molding derives every name and tag the templates interpolate.
	if resource.Resources == nil {
		return foundryerrors.Newf(foundryerrors.TypeInternal, "resource config %q carries no derived resources", resourcemolding.ResourceConfigName)
	}

	items := []struct {
		template *domain.Template
		path     string
	}{
		{versionsTFTemplate, "versions.tf.json"},
		{providersTFTemplate, "providers.tf.json"},
		{mainTFTemplate, "main.tf.json"},
		{variablesTFTemplate, "variables.tf.json"},
		{outputsTFTemplate, "outputs.tf.json"},
	}

	for _, item := range items {
		buf := bytes.NewBuffer(nil)
		if err := item.template.Execute(buf, config); err != nil {
			return foundryerrors.Wrapf(err, foundryerrors.TypeInternal, "failed to execute %s template", item.path)
		}

		p.AddJSON(buf.Bytes(), item.path)
	}

	// One cloud-init config per instance group, referenced with filebase64.
	// Blobs stay byte-exact, preserving the #cloud-config header.
	for key, group := range resource.Resources.InstanceGroups {
		buf := bytes.NewBuffer(nil)
		data := cloudInitData{
			Cluster:    resource.Resources.Cluster.Name,
			Selector:   group.Selector,
			DataVolume: group.Storage.RequiresDataVolume(),
		}

		if err := cloudInitTemplate.Execute(buf, data); err != nil {
			return foundryerrors.Wrapf(err, foundryerrors.TypeInternal, "failed to execute cloud-init template")
		}

		p.AddBlob(buf.Bytes(), "cloud-init", key+".yaml")
	}

	return nil
}

func (c *ecsEc2TerraformCasting) Cast(ctx context.Context, config infrastructure.Casting, outputPath string, p *pourer.Pourer) error {
	c.logger.WarnContext(ctx, "casting the infrastructure is not implemented yet, run terraform init and apply from the pours directory", slog.String("path", filepath.Join(outputPath, p.Dir())))
	return nil
}
