package updater

import (
	"path/filepath"
	"testing"
)

func TestDecideApplyMode(t *testing.T) {
	cases := []struct {
		name     string
		inDocker bool
		writable bool
		want     string
	}{
		{"docker tem precedência mesmo gravável", true, true, ApplyDocker},
		{"docker não gravável", true, false, ApplyDocker},
		{"binário gravável (proot/bare)", false, true, ApplySelfReplace},
		{"sem escrita", false, false, ApplyNotify},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideApplyMode(tc.inDocker, tc.writable); got != tc.want {
				t.Errorf("decideApplyMode(%v,%v) = %q, quero %q", tc.inDocker, tc.writable, got, tc.want)
			}
		})
	}
}

func TestCgroupIndicatesContainer(t *testing.T) {
	containers := []string{
		"12:devices:/docker/abc123",
		"0::/system.slice/containerd.service",
		"10:memory:/kubepods/pod123",
	}
	for _, c := range containers {
		if !cgroupIndicatesContainer(c) {
			t.Errorf("cgroupIndicatesContainer(%q) = false, quero true", c)
		}
	}

	clean := []string{
		"0::/",
		"0::/init.scope",
		"",
	}
	for _, c := range clean {
		if cgroupIndicatesContainer(c) {
			t.Errorf("cgroupIndicatesContainer(%q) = true, quero false", c)
		}
	}
}

func TestDirWritable(t *testing.T) {
	if !dirWritable(t.TempDir()) {
		t.Error("tempdir deveria ser gravável")
	}
	if dirWritable(filepath.Join(t.TempDir(), "nao", "existe")) {
		t.Error("dir inexistente não deveria ser gravável")
	}
}

func TestDetectSmoke(t *testing.T) {
	// Não deve panicar e deve produzir um ApplyMode válido.
	env := Detect("/proc/self/exe")
	switch env.ApplyMode {
	case ApplySelfReplace, ApplyDocker, ApplyNotify:
	default:
		t.Errorf("ApplyMode inválido: %q", env.ApplyMode)
	}
}
