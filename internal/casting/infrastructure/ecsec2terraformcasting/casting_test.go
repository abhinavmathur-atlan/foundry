package ecsec2terraformcasting

import (
	"context"
	"encoding/json"
	awsconvention "github.com/signoz/foundry/internal/convention/aws"
	"log/slog"
	"testing"

	"github.com/signoz/foundry/api/v1alpha1"
	"github.com/signoz/foundry/api/v1alpha1/infrastructure"
	"github.com/signoz/foundry/internal/domain"
	"github.com/signoz/foundry/internal/molding/infrastructure/resourcemolding"
	"github.com/signoz/foundry/internal/pourer"
	"github.com/stretchr/testify/assert"
)

// A zone is per-account and per-region, so the substrate cannot be described
// without the operator stating one.
const subnets = `networking:
  subnets:
    private-a: {type: private, zone: us-east-1a, cidr: 10.0.0.0/19}
    public-a: {type: public, zone: us-east-1a, cidr: 10.0.96.0/22}
`

// moldedCasting runs the enricher and the molding, which is what puts the
// derived names and tags the templates interpolate into the status.
func moldedCasting(t *testing.T) *infrastructure.Casting {
	t.Helper()

	config := infrastructure.Default()
	config.Spec.Resource.Spec.Config.Set(resourcemolding.ResourceConfigName, []byte(subnets))

	logger := slog.New(slog.DiscardHandler)
	assert.NoError(t, newEcsEc2TerraformMoldingEnricher().EnrichStatus(context.Background(), v1alpha1.MoldingKindResource, config))
	assert.NoError(t, resourcemolding.New(logger, awsconvention.Resources).MoldV1Alpha1(context.Background(), config))

	return config
}

func TestForge(t *testing.T) {
	config := moldedCasting(t)

	pr := pourer.New("infrastructure")
	assert.NoError(t, New(slog.New(slog.DiscardHandler)).Forge(context.Background(), *config, pr))

	materials, err := pr.Pour()
	assert.NoError(t, err)

	paths := map[string]domain.Material{}
	for _, material := range materials {
		paths[material.Path()] = material
	}

	for _, expected := range []string{
		"infrastructure/providers.tf.json",
		"infrastructure/main.tf.json",
		"infrastructure/variables.tf.json",
		"infrastructure/outputs.tf.json",
		"infrastructure/cloud-init/persistent.yaml",
		"infrastructure/cloud-init/ephemeral.yaml",
	} {
		assert.Contains(t, paths, expected)
	}

	pinned := string(paths["infrastructure/cloud-init/persistent.yaml"].FmtContents())
	assert.Contains(t, pinned, "ECS_CLUSTER=signoz-cls")
	assert.Contains(t, pinned, `"foundry.signoz.io/storage":"persistent"`)
	assert.Contains(t, pinned, "/var/lib/foundry")

	pool := string(paths["infrastructure/cloud-init/ephemeral.yaml"].FmtContents())
	assert.Contains(t, pool, `"foundry.signoz.io/storage":"ephemeral"`)
	assert.NotContains(t, pool, "fs_setup")
}

// The templates interpolate the derived document and assemble no name or tag
// of their own, so nothing in them can drift from what a consumer filters on.
func TestForge_TemplatesSpellNoContractOfTheirOwn(t *testing.T) {
	config := moldedCasting(t)

	pr := pourer.New("infrastructure")
	assert.NoError(t, New(slog.New(slog.DiscardHandler)).Forge(context.Background(), *config, pr))

	materials, err := pr.Pour()
	assert.NoError(t, err)

	resource := &infrastructure.ResourceConfig{}
	assert.NoError(t, domain.UnmarshalYAML([]byte(config.Spec.Resource.Status.Config.Data[resourcemolding.ResourceConfigName]), resource))

	main := map[string]any{}
	for _, material := range materials {
		if material.Path() == "infrastructure/main.tf.json" {
			assert.NoError(t, json.Unmarshal(material.FmtContents(), &main))
		}
	}

	resources, _ := main["resource"].(map[string]any)
	subnet, _ := resources["aws_subnet"].(map[string]any)
	private, _ := subnet["private-a"].(map[string]any)

	assert.Equal(t, toAny(resource.Resources.Subnets["private-a"].Tags), private["tags"])

	cluster, _ := resources["aws_ecs_cluster"].(map[string]any)
	main0, _ := cluster["main"].(map[string]any)

	assert.Equal(t, resource.Resources.Cluster.Name, main0["name"])
	assert.Equal(t, toAny(resource.Resources.Cluster.Tags), main0["tags"])
}

func toAny(tags map[string]string) map[string]any {
	out := map[string]any{}
	for key, value := range tags {
		out[key] = value
	}

	return out
}

func TestForge_MissingResourceConfigErrors(t *testing.T) {
	config := infrastructure.Default()

	err := New(slog.New(slog.DiscardHandler)).Forge(context.Background(), *config, pourer.New("infrastructure"))
	assert.Error(t, err)
}

// Only the molding derives, so a status carrying a declaration and nothing
// else has no names for the templates to interpolate.
func TestForge_UnderivedResourceConfigErrors(t *testing.T) {
	config := infrastructure.Default()
	config.Spec.Resource.Status.Config.Set(resourcemolding.ResourceConfigName, []byte(subnets))

	err := New(slog.New(slog.DiscardHandler)).Forge(context.Background(), *config, pourer.New("infrastructure"))
	assert.Error(t, err)
}
