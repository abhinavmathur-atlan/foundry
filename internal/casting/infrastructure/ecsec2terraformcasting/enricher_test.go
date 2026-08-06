package ecsec2terraformcasting

import (
	"context"
	"testing"

	"github.com/signoz/foundry/api/v1alpha1"
	"github.com/signoz/foundry/api/v1alpha1/infrastructure"
	"github.com/signoz/foundry/internal/domain"
	"github.com/signoz/foundry/internal/molding/infrastructure/resourcemolding"
	"github.com/stretchr/testify/assert"
)

func TestEnrichStatus(t *testing.T) {
	tests := []struct {
		name   string
		kind   v1alpha1.MoldingKind
		assert func(t *testing.T, config *infrastructure.Casting)
	}{
		{
			name: "ResourceMolding_ContributesBothGroups",
			kind: v1alpha1.MoldingKindResource,
			assert: func(t *testing.T, config *infrastructure.Casting) {
				doc := config.Spec.Resource.Status.Config.Data[resourcemolding.ResourceConfigName]
				assert.NotEmpty(t, doc)

				resolved := &infrastructure.ResourceConfig{}
				assert.NoError(t, domain.UnmarshalYAML([]byte(doc), resolved))
				assert.Len(t, resolved.InstanceGroups, 2)
				assert.Equal(t, machineTypePersistent, resolved.InstanceGroups[resourcemolding.GroupPersistent].MachineType)
				assert.Equal(t, machineTypeEphemeral, resolved.InstanceGroups[resourcemolding.GroupEphemeral].MachineType)
				assert.Equal(t, volumeType, resolved.InstanceGroups[resourcemolding.GroupPersistent].DataVolume.Type)
			},
		},
		{
			// The contribution is a delta: sizes belong to the molding's
			// baseline, so stating one here would override it.
			name: "ResourceMolding_StatesNoSizes",
			kind: v1alpha1.MoldingKindResource,
			assert: func(t *testing.T, config *infrastructure.Casting) {
				doc := config.Spec.Resource.Status.Config.Data[resourcemolding.ResourceConfigName]

				assert.NotContains(t, doc, "minSize")
				assert.NotContains(t, doc, "size")
				assert.NotContains(t, doc, "subnets")
			},
		},
		{
			name: "OtherMolding_NoOp",
			kind: v1alpha1.MoldingKindTelemetryStore,
			assert: func(t *testing.T, config *infrastructure.Casting) {
				assert.Empty(t, config.Spec.Resource.Status.Config.Data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := infrastructure.Default()

			err := newEcsEc2TerraformMoldingEnricher().EnrichStatus(context.Background(), tt.kind, config)
			assert.NoError(t, err)
			tt.assert(t, config)
		})
	}
}
