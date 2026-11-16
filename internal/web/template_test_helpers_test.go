package web

import "testing"

func readEmbeddedTemplate(t *testing.T, name string) string {
	t.Helper()
	content, err := templateFS.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", name, err)
	}
	return string(content)
}
