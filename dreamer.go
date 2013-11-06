package main

import (
	"code.google.com/p/gorest"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/fcgi"
	"spicerack"
)

var (
	fastcgi                = flag.Bool("fcgi", false, "Run under FastCGI mode")
	dbUser, dbPass, dbName string
)

func main() {
	flag.Parse()
	loadConfig()
	gorest.RegisterService(new(DreamService))

	if !*fastcgi {
		fmt.Println("Running Locally")
		http.Handle("/", gorest.Handle())
		fmt.Println(http.ListenAndServe(":9000", nil))
	} else {
		fmt.Println("Running as FastCGI")
		l, _ := net.Listen("tcp", ":9000")
		fmt.Println(fcgi.Serve(l, gorest.Handle()))
	}
}

func loadConfig() {
	conf, _ := spicerack.GofigFromEnv("ME_CONF")
	salty, _ := conf.Map("salty")
	dbUser = salty["db_user"].(string)
	dbName = salty["db_name"].(string)
	dbPass = salty["db_pass"].(string)
}

type DreamService struct {
	gorest.RestService `root:"/api" consumes:"application/json" produces:"application/json"`

	getHistory gorest.EndPoint `method:"GET" path:"/h/{Name:string}" output:"spicerack.History"`
}

func (serv DreamService) GetHistory(Name string) (h spicerack.History) {
	db := spicerack.Db(dbUser, dbPass, dbName)
	defer db.Close()

	f, err := db.GetFighter(Name)
	if err != nil || f.Id == 0 {
		serv.ResponseBuilder().SetResponseCode(404)
		return
	}

	h = *db.GetHistory(f)
	serv.ResponseBuilder().SetResponseCode(200)
	return
}
