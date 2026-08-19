package whatsmiau

import (
	"testing"

	"github.com/verbeux-ai/whatsmiau/models"
)

func TestShouldSaveMediaDefaultsToEnabledWhenUnset(t *testing.T) {
	instance := &models.Instance{}

	if !shouldSaveMedia(instance) {
		t.Error("shouldSaveMedia(nil SaveMedia) = false, want true: instances created before the flag existed must keep persisting media")
	}
}

func TestShouldSaveMediaRespectsExplicitValues(t *testing.T) {
	enabled, disabled := true, false

	if !shouldSaveMedia(&models.Instance{SaveMedia: &enabled}) {
		t.Error("shouldSaveMedia(SaveMedia=true) = false, want true")
	}

	if shouldSaveMedia(&models.Instance{SaveMedia: &disabled}) {
		t.Error("shouldSaveMedia(SaveMedia=false) = true, want false")
	}
}
