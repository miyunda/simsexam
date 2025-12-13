package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	cwd, _ := filepath.Abs(".")
	files := []string{
		filepath.Join(cwd, "templates/layouts/base.html"),
		filepath.Join(cwd, "templates/views", tmpl),
	}

	ts, err := template.ParseFiles(files...)
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
