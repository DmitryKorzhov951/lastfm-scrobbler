package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestТаЖеСборка(t *testing.T) {
	cur := currentAbout()
	var reply any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/about" {
			http.NotFound(w, r)
			return
		}
		if reply == nil {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(reply)
	}))
	defer srv.Close()
	u := srv.URL + "/#t=ключ"

	if sameBuild(u) {
		t.Error("старая сборка без /about принята за ту же")
	}
	reply = appAbout{Version: cur.Version, Dir: strings.ToUpper(cur.Dir)}
	if !sameBuild(u) {
		t.Error("та же сборка не узнана (регистр пути не должен мешать)")
	}
	reply = appAbout{Version: cur.Version + "-старая", Dir: cur.Dir}
	if sameBuild(u) {
		t.Error("другая версия принята за ту же")
	}
	reply = appAbout{Version: cur.Version, Dir: cur.Dir + "-копия"}
	if sameBuild(u) {
		t.Error("копия из другой папки принята за ту же")
	}
	if sameBuild("http://127.0.0.1:1/#t=x") {
		t.Error("недоступная копия принята за ту же")
	}
}
