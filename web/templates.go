package web

import (
	"bytes"
	"embed"
	"html/template"
	"io"
)

//go:embed templates
var templateFS embed.FS

type Renderer struct {
	pages     map[string]*template.Template
	fragments *template.Template
}

func NewRenderer() (*Renderer, error) {
	base, err := template.New("base").ParseFS(templateFS, "templates/layout.html", "templates/partials/*.html")
	if err != nil {
		return nil, err
	}
	r := &Renderer{pages: make(map[string]*template.Template), fragments: base}
	for _, page := range []string{"catalog", "book_form", "book_detail", "login", "register", "error"} {
		t, err := base.Clone()
		if err != nil {
			return nil, err
		}
		t, err = t.ParseFS(templateFS, "templates/pages/"+page+".html")
		if err != nil {
			return nil, err
		}
		r.pages[page] = t
	}
	return r, nil
}

// Render buffers output so a template error cannot send a partial success page.
func (r *Renderer) Render(w io.Writer, name string, data any) error {
	t, entry := r.fragments, name
	if page, ok := r.pages[name]; ok {
		t, entry = page, "layout"
	}
	var b bytes.Buffer
	if err := t.ExecuteTemplate(&b, entry, data); err != nil {
		return err
	}
	_, err := b.WriteTo(w)
	return err
}
