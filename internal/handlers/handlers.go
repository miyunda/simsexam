package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"simsexam/internal/buildinfo"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	markdownhtml "github.com/gomarkdown/markdown/html"
)

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	cwd, _ := filepath.Abs(".")
	files := []string{
		filepath.Join(cwd, "templates/layouts/base.html"),
		filepath.Join(cwd, "templates/views", tmpl),
	}

	ts, err := template.New("base").Funcs(template.FuncMap{
		"footerVersion": buildinfo.FooterSummary,
		"markdown":      renderMarkdown,
	}).ParseFiles(files...)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
		return
	}

	err = ts.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal Server Error", 500)
	}
}

var codeLanguagePattern = regexp.MustCompile(`^[a-zA-Z0-9_+.#-]+$`)

// Only the renderer's escaped output is trusted. Raw HTML and images remain
// disabled; image hosting is a separate, deferred feature.
func renderMarkdown(source string) template.HTML {
	renderer := markdownhtml.NewRenderer(markdownhtml.RendererOptions{
		Flags: markdownhtml.SkipHTML | markdownhtml.SkipImages | markdownhtml.Safelink,
	})
	document := markdown.Parse([]byte(source), nil)
	ast.WalkFunc(document, func(node ast.Node, entering bool) ast.WalkStatus {
		if code, ok := node.(*ast.CodeBlock); ok && entering {
			// The library writes language metadata into an HTML attribute verbatim.
			// Keep only a simple language identifier before trusting its output.
			fields := strings.Fields(string(code.Info))
			code.Info = nil
			if len(fields) > 0 && codeLanguagePattern.MatchString(fields[0]) {
				code.Info = []byte(fields[0])
			}
		}
		return ast.GoToNext
	})
	return template.HTML(markdown.Render(document, renderer))
}
