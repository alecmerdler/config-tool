package editor

import (
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"path"

	auth "github.com/abbot/go-http-auth"
	bcrypt "golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"

	jsoniter "github.com/json-iterator/go"
	config "github.com/quay/config-tool/pkg/lib/config"
	shared "github.com/quay/config-tool/pkg/lib/shared"
)

const port = 8080
const staticContentPath = "pkg/lib/editor/static"
const editorUsername = "quayconfig"

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		p := staticContentPath + "/index.html"
		http.ServeFile(w, r, p)
		return
	}

	w.WriteHeader(404)
}

func configValidator(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(404)
		return
	}

	var c map[string]interface{}
	err := yaml.NewDecoder(r.Body).Decode(&c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	loaded, err := config.NewConfig(c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	opts := shared.Options{
		Mode: "online",
	}

	errors := loaded.Validate(opts)

	for _, err := range errors {
		fmt.Println(err)
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	js, err := json.Marshal(errors)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(js)

	// FIXME(alecmerdler): Set this to Operator endpoint...
	_, err = http.Post("https://webhook.site/5699f67f-17a0-466c-9174-23dc30109c47", "application/json", r.Body)
	if err != nil {
		panic(err)
	}
}

func configHandler(configPath string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(404)
			return
		}

		// Read config file
		configFilePath := path.Join(configPath, "config.yaml")
		configBytes, err := ioutil.ReadFile(configFilePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Load config into struct
		var c map[string]interface{}
		if err = yaml.Unmarshal(configBytes, &c); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		js, err := json.Marshal(c)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.Write(js)
	}
}

// RunConfigEditor runs the configuration editor server.
func RunConfigEditor(password string, configPath string) {
	mime.AddExtensionType(".css", "text/css; charset=utf-8")
	mime.AddExtensionType(".js", "application/javascript; charset=utf-8")

	log.Printf("Running the configuration editor on port %v with username %s", port, editorUsername)

	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), 5)
	authenticator := auth.NewBasicAuthenticator(editorUsername, func(user, realm string) string {
		if user == editorUsername {
			return string(hashed)
		}
		return ""
	})

	http.HandleFunc("/", auth.JustCheck(authenticator, handler))
	http.HandleFunc("/api/v1/config", auth.JustCheck(authenticator, configHandler(configPath)))
	http.HandleFunc("/api/v1/config/validate", auth.JustCheck(authenticator, configValidator))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticContentPath))))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
}
