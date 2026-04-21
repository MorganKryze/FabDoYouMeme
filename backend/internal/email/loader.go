// backend/internal/email/loader.go
package email

import (
	"encoding/json"
	"fmt"
	ht "html/template"
	"io/fs"
	"path"
	"strings"
	tt "text/template"
)

// localeTemplates holds every template (html + txt) and every subject line
// for a single locale. One of these per supported language.
type localeTemplates struct {
	HTML     *ht.Template
	Text     *tt.Template
	Subjects map[string]string
}

// loadTemplateTree walks templates/<locale>/ for each supplied locale,
// parses all .html into one html/template tree and all .txt into one
// text/template tree per locale, and loads subjects.json.
//
// Parity contract: every non-subjects template name present in the first
// (reference) locale must be present in every other locale, and vice
// versa. Any divergence is a hard error — it is only caught at startup,
// so a missing FR template cannot silently fall back to EN in production.
func loadTemplateTree(tfs fs.FS, locales []string) (map[string]*localeTemplates, error) {
	if len(locales) == 0 {
		return nil, fmt.Errorf("email: no locales")
	}

	out := make(map[string]*localeTemplates, len(locales))
	var reference map[string]struct{}

	for _, loc := range locales {
		dir := path.Join("templates", loc)
		entries, err := fs.ReadDir(tfs, dir)
		if err != nil {
			return nil, fmt.Errorf("email: read %s: %w", dir, err)
		}

		names := map[string]struct{}{}
		var htmlFiles, txtFiles []string
		var subjectsRaw []byte

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			full := path.Join(dir, name)
			switch {
			case name == "subjects.json":
				subjectsRaw, err = fs.ReadFile(tfs, full)
				if err != nil {
					return nil, fmt.Errorf("email: read %s: %w", full, err)
				}
			case strings.HasSuffix(name, ".html"):
				htmlFiles = append(htmlFiles, full)
				names[strings.TrimSuffix(name, ".html")] = struct{}{}
			case strings.HasSuffix(name, ".txt"):
				txtFiles = append(txtFiles, full)
				names[strings.TrimSuffix(name, ".txt")] = struct{}{}
			}
		}

		if reference == nil {
			reference = names
		} else {
			for k := range reference {
				if _, ok := names[k]; !ok {
					return nil, fmt.Errorf("email: locale %s is missing template %q present in reference locale", loc, k)
				}
			}
			for k := range names {
				if _, ok := reference[k]; !ok {
					return nil, fmt.Errorf("email: locale %s has extra template %q not in reference locale", loc, k)
				}
			}
		}

		htmlTmpl := ht.New("")
		for _, f := range htmlFiles {
			data, err := fs.ReadFile(tfs, f)
			if err != nil {
				return nil, fmt.Errorf("email: read %s: %w", f, err)
			}
			if _, err := htmlTmpl.New(path.Base(f)).Parse(string(data)); err != nil {
				return nil, fmt.Errorf("email: parse %s: %w", f, err)
			}
		}

		txtTmpl := tt.New("")
		for _, f := range txtFiles {
			data, err := fs.ReadFile(tfs, f)
			if err != nil {
				return nil, fmt.Errorf("email: read %s: %w", f, err)
			}
			if _, err := txtTmpl.New(path.Base(f)).Parse(string(data)); err != nil {
				return nil, fmt.Errorf("email: parse %s: %w", f, err)
			}
		}

		subjects := map[string]string{}
		if len(subjectsRaw) > 0 {
			if err := json.Unmarshal(subjectsRaw, &subjects); err != nil {
				return nil, fmt.Errorf("email: parse %s/subjects.json: %w", dir, err)
			}
		}

		out[loc] = &localeTemplates{
			HTML:     htmlTmpl,
			Text:     txtTmpl,
			Subjects: subjects,
		}
	}

	return out, nil
}
