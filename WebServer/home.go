package WebServer

import (
	"fmt"
	"path"
	"net/http"
	"html/template"
)

func init() {
	router.HandleFunc("/", homePage)
}

func homePage(w http.ResponseWriter, r *http.Request) {
	serverConfig := GetServerConfig(r)
	mainTemplate := path.Join(serverConfig.templateDir, "main.tmpl")
	fmt.Println(mainTemplate)
    t, err := template.ParseFiles(mainTemplate)
    if err != nil {
    	panic("could not open template file")
    }
    t.Execute(w, fmt.Sprintf("%d", serverConfig.port))
}
