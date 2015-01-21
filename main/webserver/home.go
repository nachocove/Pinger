package main

import (
	"fmt"
	"html/template"
	"net/http"
	"path"
)

func init() {
	httpsRouter.HandleFunc("/", homePage)
}
func homePage(w http.ResponseWriter, r *http.Request) {
	context := GetContext(r)
	config := context.Config
	mainTemplate := path.Join(config.Server.TemplateDir, "main.tmpl")
	fmt.Println(mainTemplate)
	t, err := template.ParseFiles(mainTemplate)
	if err != nil {
		panic("could not open template file")
	}
	t.Execute(w, fmt.Sprintf("%d", config.Server.Port))
}
