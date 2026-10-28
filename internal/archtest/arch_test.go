package archtest

import (
	"os/exec"
	"strings"
	"testing"
)

func imports(t *testing.T, pkg string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", "{{range .Imports}}{{.}}\n{{end}}", pkg).Output()
	if err != nil {
		t.Fatalf("go list %s: %v", pkg, err)
	}
	var imports []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			imports = append(imports, line)
		}
	}
	return imports
}

func TestSDKHasNoProjectImports(t *testing.T) {
	for _, imp := range imports(t, "icarosu/internal/sdk") {
		if strings.HasPrefix(imp, "icarosu/internal") {
			t.Fatalf("sdk must not import project packages, got %q", imp)
		}
	}
}

func TestKernelDoesNotImportPlugins(t *testing.T) {
	for _, imp := range imports(t, "icarosu/internal/kernel") {
		if strings.HasPrefix(imp, "icarosu/internal/plugins") {
			t.Fatalf("kernel must not import plugins, got %q", imp)
		}
	}
}

func TestFirstPartyPluginDoesNotImportKernelOrOtherPlugins(t *testing.T) {
	const self = "icarosu/internal/plugins/tool/time"
	for _, imp := range imports(t, self) {
		if strings.HasPrefix(imp, "icarosu/internal/kernel") {
			t.Fatalf("plugin must not import kernel, got %q", imp)
		}
		if strings.HasPrefix(imp, "icarosu/internal/plugins/") && imp != self {
			t.Fatalf("plugin must not import other plugins, got %q", imp)
		}
	}
}
