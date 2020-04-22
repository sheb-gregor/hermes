package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	HostPort  string              `yaml:"host_port"`
	AllowAll  bool                `yaml:"allow_all"`
	WhiteList map[string]RoleDef  `yaml:"white_list"`
	BlackList map[string]struct{} `yaml:"black_list"`
}

type RoleDef struct {
	UserID string `yaml:"user_id"`
	Role   string `yaml:"role"`
}

type authRequest struct {
	SessionID int64  `json:"session_id"`
	Token     string `json:"token"`
	UserAgent string `json:"userAgent"`
	IP        string `json:"ip"`
	Origin    string `json:"origin"`
	Role      string `json:"role"`
}

type authResponse struct {
	UserID string `json:"userId"`
	TTL    int64  `json:"ttl"`
}

func main() {
	configPath := flag.String("conf", "config.yaml", "path to config file")
	flag.Parse()

	file, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Println("[Error] ", errors.Wrap(err, "unable to open config").Error())
		return
	}

	var conf = Config{}
	err = yaml.Unmarshal(file, &conf)
	if err != nil {
		log.Println("[Error] ", errors.Wrap(err, "unable to parse config").Error())
		return
	}

	http.HandleFunc("/validate-session", func(w http.ResponseWriter, r *http.Request) {

		request := new(authRequest)

		err := json.NewDecoder(r.Body).Decode(request)
		if err != nil {
			log.Print("[ERROR] ", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if conf.AllowAll {
			raw, _ := json.Marshal(authResponse{UserID: request.Token, TTL: -1})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(raw)
			return
		}

		if _, ok := conf.BlackList[request.Token]; ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		def, ok := conf.WhiteList[request.Token]
		if ok && def.Role == request.Role {
			raw, _ := json.Marshal(authResponse{UserID: def.UserID, TTL: -1})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(raw)
			return
		}

		w.WriteHeader(http.StatusUnauthorized)
		return

	})

	_ = http.ListenAndServe(conf.HostPort, nil)
}
