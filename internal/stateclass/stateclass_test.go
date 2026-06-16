package stateclass_test

import (
	"testing"

	"camera/internal/stateclass"
)

func valid() stateclass.Classifier {
	return stateclass.Classifier{
		Name:          "Portão",
		Threshold:     0.8,
		TriggerMotion: true,
		CropX:         0.1, CropY: 0.1, CropW: 0.3, CropH: 0.3,
		Classes: []string{"aberto", "fechado"},
	}
}

func TestValidateAcceptsValid(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("esperava válido, got %v", err)
	}
}

func TestValidateRejectsEmptyName(t *testing.T) {
	c := valid()
	c.Name = "   "
	if err := c.Validate(); err == nil {
		t.Fatal("esperava erro de nome vazio")
	}
}

func TestValidateRequiresTwoClasses(t *testing.T) {
	c := valid()
	c.Classes = []string{"aberto"}
	if err := c.Validate(); err == nil {
		t.Fatal("esperava erro de < 2 classes")
	}
}

func TestValidateRejectsBadCrop(t *testing.T) {
	cases := []stateclass.Classifier{}
	c := valid()
	c.CropW = 0
	cases = append(cases, c)
	c = valid()
	c.CropX = 0.8
	c.CropW = 0.5 // x+w > 1
	cases = append(cases, c)
	c = valid()
	c.CropY = -0.1
	cases = append(cases, c)
	for i, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Fatalf("caso %d: esperava erro de crop inválido", i)
		}
	}
}

func TestValidateRequiresATrigger(t *testing.T) {
	c := valid()
	c.TriggerMotion = false
	c.TriggerIntervalSeconds = 0
	if err := c.Validate(); err == nil {
		t.Fatal("esperava erro de ausência de gatilho")
	}
}
