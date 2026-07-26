package service

import "testing"

func TestValidateCartoonStrength(t *testing.T) {
	valid := []string{
		`portrait`,
		`{"cartoonization_strength": 0}`,
		`{"cartoonization_strength": 0.75}`,
	}
	for _, value := range valid {
		if err := ValidateCartoonStrength(value); err != nil {
			t.Fatalf("%s should be valid: %v", value, err)
		}
	}
	invalid := []string{
		`{"cartoonization_strength": 2}`,
		`{"cartoonization_strength": "0.5"}`,
		`{"cartoonization_strength": 0, "cartoonization_strength": 1}`,
	}
	for _, value := range invalid {
		if err := ValidateCartoonStrength(value); err == nil {
			t.Fatalf("%s should be invalid", value)
		}
	}
}

func TestNodeDimensions(t *testing.T) {
	width, height := nodeDimensions("16:9")
	if width != 360 || height >= width {
		t.Fatalf("unexpected dimensions %v x %v", width, height)
	}
}

func TestPromptActionNameDoesNotRequireSeparateEmployeeLabel(t *testing.T) {
	if got := promptActionName("", "cartoonize", ""); got != "cartoonize" {
		t.Fatalf("new actions should default to action_key, got %q", got)
	}
	if got := promptActionName("", "cartoonize-v2", "卡通化"); got != "卡通化" {
		t.Fatalf("editing should preserve the existing display name, got %q", got)
	}
	if got := promptActionName("  新名称  ", "cartoonize", "旧名称"); got != "新名称" {
		t.Fatalf("an explicitly provided name should still be honored, got %q", got)
	}
}
