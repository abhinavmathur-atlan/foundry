package ecsec2terraformcasting

import (
	"testing"

	"github.com/signoz/foundry/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestTemplates_RenderValidJSON(t *testing.T) {
	config := moldedCasting(t)

	tests := []struct {
		name     string
		template *domain.Template
	}{
		{name: "ProvidersTemplate_RendersValidJSON", template: providersTFTemplate},
		{name: "MainTemplate_RendersValidJSON", template: mainTFTemplate},
		{name: "VariablesTemplate_RendersValidJSON", template: variablesTFTemplate},
		{name: "OutputsTemplate_RendersValidJSON", template: outputsTFTemplate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			material, err := tt.template.Render(*config, "out.tf.json")
			assert.NoError(t, err)
			assert.NotEmpty(t, material.FmtContents())
		})
	}
}

func TestMainTemplate_PinsPersistentAndPoolsEphemeral(t *testing.T) {
	config := moldedCasting(t)

	material, err := mainTFTemplate.Render(*config, "out.tf.json")
	assert.NoError(t, err)

	contents := string(material.FmtContents())
	assert.Contains(t, contents, `"persistent-0"`)
	assert.Contains(t, contents, `"persistent-2"`)
	assert.Contains(t, contents, `"aws_ebs_volume"`)
	assert.Contains(t, contents, `"aws_volume_attachment"`)
	assert.Contains(t, contents, `"aws_autoscaling_group"`)
	assert.Contains(t, contents, "cloud-init/persistent.yaml")
	assert.Contains(t, contents, "cloud-init/ephemeral.yaml")
	assert.NotContains(t, contents, "aws_launch_template.persistent")
	assert.NotContains(t, contents, "aws_instance.ephemeral")
}
