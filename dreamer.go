package main

import (
	"code.google.com/p/gorest"
	"flag"
	"net"
	"net/http"
	"net/http/fcgi"
	"spicerack"
)

var fastcgi = flag.Bool("fcgi", false, "Run under FastCGI mode")

func main() {
	flag.Parse()
	gorest.RegisterService(new(DreamService))

	if !*fastcgi {
		http.Handle("/", gorest.Handle())
		fmt.Println(http.ListenAndServe(":4380", nil))
	} else {
		l, _ := net.Listen("tcp", ":4380")
		fmt.Println(fcgi.Serve(l, gorest.Handle()))
	}
}

type DreamService struct {
	gorest.RestService `root:"/" consumes:"application/json" produces:"application/json"`

	getHistory gorest.EndPoint `method:"GET" path:"/h/{Name:string}" output:"spicerack.History"`
}

func (serv DreamService) GetHistory(Name string) (h spicerack.History) {
	db, err := spicerack.OpenDb("postgres", "_N3rd3ry", "dreams")
	if err != nil {
		serv.ResponseBuilder().SetResponseCode(500)
		return
	}
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
