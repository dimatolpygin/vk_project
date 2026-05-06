package handlers

import "html/template"

var tmplFuncs = template.FuncMap{
	"inc": func(n int) int { return n + 1 },
	"dec": func(n int) int { return n - 1 },
}

func parseTemplates(files ...string) *template.Template {
	return template.Must(template.New("").Funcs(tmplFuncs).ParseFiles(files...))
}
