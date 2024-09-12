package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/publicsuffix"
)

type WebLink struct {
	Name string
	URL  string
}

type TemplateData struct {
	Links []*Server
	//Sites []WebLink
	//Webchats []WebLink
	/* I'll make these work eventually lol */
}

type Server struct {
	Name               string
	UpstreamServerName string
	UpstreamServer     *Server
	HopCount           int
	Info               string
}

func (s *Server) String() string {
	return fmt.Sprintf("%q -> %q (%p) (%d) %s", s.Name, s.UpstreamServerName, s.UpstreamServer, s.HopCount, s.Info)
}

func main() {
	var activeServers []*Server
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get("https://plas.netsplit.nl/data/api/v1/links")
	if err != nil {
		panic(err)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	res := ParseJSON(data)
	for _, s := range res {
		//fmt.Println(s)
		eTLD, icann := publicsuffix.PublicSuffix(s.Name)
		if icann || strings.IndexByte(eTLD, '.') >= 0 {
			_, err := net.LookupIP(s.Name)
			if err == nil {
				activeServers = append(activeServers, s)
			}
		}
	}

	f := makeFolderAndFile()
	defer f.Close()

	tmpl := template.Must(template.ParseFiles("template.html"))
	tmpl.Execute(f, &TemplateData{Links: activeServers})
}

func makeFolderAndFile() *os.File {
	if _, err := os.Stat("./public"); os.IsNotExist(err) {
		os.Mkdir("./public", os.ModeDir|0755)
	}
	f, err := os.OpenFile("./public/index.html", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		panic(err)
	}

	return f
}

func ParseJSON(data []byte) []*Server {
	type GrossServerBase struct {
		Name         string `json:"server_name"`
		UpstreamName string `json:"upstream_server_name"`
		HopCount     string `json:"hop_count"`
		Info         string `json:"info"`
	}
	var temp struct {
		Links map[string]GrossServerBase `json:"links"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		panic(err)
	}
	out := make([]*Server, 0, len(temp.Links))

	for n, s := range temp.Links {
		hopCount, err := strconv.Atoi(s.HopCount)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Unexpected error while parsing hopcount: ", err)
			hopCount = -1
		}
		realServer := &Server{
			Name:               s.Name,
			UpstreamServerName: s.UpstreamName,
			UpstreamServer:     nil,
			HopCount:           hopCount,
			Info:               s.Info,
		}
		out = append(out, realServer)
		if n != s.Name {
			fmt.Fprintf(os.Stderr, "Server names are unexpectedly nonequal: %s != %s\n", n, s.Name)
		}
	}

	for _, s := range out {
		for _, other := range out {
			if s.UpstreamServerName == other.Name {
				s.UpstreamServer = other
				break
			}
		}
	}

	return out
}
