package email

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadTemplateTree_parityMissingLocale(t *testing.T) {
	mfs := fstest.MapFS{
		"templates/en/magic_link_login.html": {Data: []byte(`<p>{{.Token}}</p>`)},
		"templates/en/magic_link_login.txt":  {Data: []byte(`{{.Token}}`)},
		"templates/en/subjects.json":         {Data: []byte(`{"magic_link_login":"Your login link"}`)},
		// fr directory intentionally empty — should fail parity
		"templates/fr/subjects.json": {Data: []byte(`{}`)},
	}
	_, err := loadTemplateTree(mfs, []string{"en", "fr"})
	if err == nil {
		t.Fatal("expected parity failure, got nil")
	}
	if !strings.Contains(err.Error(), "missing template") {
		t.Fatalf("expected 'missing template' in error, got %q", err)
	}
}

func TestLoadTemplateTree_ok(t *testing.T) {
	mfs := fstest.MapFS{
		"templates/en/magic_link_login.html": {Data: []byte(`<p>{{.Token}}</p>`)},
		"templates/en/magic_link_login.txt":  {Data: []byte(`{{.Token}}`)},
		"templates/en/subjects.json":         {Data: []byte(`{"magic_link_login":"Your login link"}`)},
		"templates/fr/magic_link_login.html": {Data: []byte(`<p>[FR] {{.Token}}</p>`)},
		"templates/fr/magic_link_login.txt":  {Data: []byte(`[FR] {{.Token}}`)},
		"templates/fr/subjects.json":         {Data: []byte(`{"magic_link_login":"[FR] Your login link"}`)},
	}
	tree, err := loadTemplateTree(mfs, []string{"en", "fr"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tree["en"]; !ok {
		t.Fatal("tree missing en")
	}
	if _, ok := tree["fr"]; !ok {
		t.Fatal("tree missing fr")
	}
	if tree["en"].Subjects["magic_link_login"] != "Your login link" {
		t.Fatalf("en subject mismatch: %q", tree["en"].Subjects["magic_link_login"])
	}
}
