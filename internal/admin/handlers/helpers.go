package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
)

type ctxKey string

const AdminBaseKey ctxKey = "adminBase"

func GetAdminBase(r *http.Request) string {
	if base, ok := r.Context().Value(AdminBaseKey).(string); ok && base != "" {
		return base
	}
	return "/admin"
}

var tmplFuncs = template.FuncMap{
	"inc": func(n int) int { return n + 1 },
	"dec": func(n int) int { return n - 1 },
	"toJSON": func(v any) template.JS {
		b, err := json.Marshal(v)
		if err != nil {
			return template.JS("null")
		}
		return template.JS(b)
	},
}

func parseTemplates(files ...string) *template.Template {
	return template.Must(template.New("").Funcs(tmplFuncs).ParseFiles(files...))
}
