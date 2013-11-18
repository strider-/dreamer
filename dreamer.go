package main

/*
   JSON API service that returns the detailed win/loss records for the current fight card.
   Acts either as a FastCGI listener (reverse proxy for Nginx or Apache), or a local webserver.
   TODO:
   		 -Maybe make the port a parameter?
	 	 -Create an Upstart conf instead of using nohup & detatching from the terminal
*/

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
	illumEmail, illumPass  string
	theShiznit, statsUrl   string
	webClient              *http.Client
)

func main() {
	flag.Parse()
	loadConfig()
	gorest.RegisterService(new(DreamService))
	var err error

	webClient, err = spicerack.LogIntoSaltyBet(illumEmail, illumPass)
	if err != nil {
		fmt.Printf("Error logging into Salty Bet: %v\n", err)
	}

	if !*fastcgi {
		fmt.Println("Running Locally")
		http.HandleFunc("/index", homePage)
		http.HandleFunc("/ds.js", homePage)
		http.Handle("/", gorest.Handle())
		fmt.Println(http.ListenAndServe(":9000", nil))
	} else {
		fmt.Println("Running as FastCGI")
		l, _ := net.Listen("tcp", ":9000")
		fmt.Println(fcgi.Serve(l, gorest.Handle()))
	}
}

func homePage(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[1:]
	if file == "index" {
		file += ".html"
	}
	http.ServeFile(w, r, file)
}

func loadConfig() {
	conf, _ := spicerack.GofigFromEnv("ME_CONF")
	salty, _ := conf.Map("salty")
	dbUser = salty["db_user"].(string)
	dbName = salty["db_name"].(string)
	dbPass = salty["db_pass"].(string)
	illumEmail = salty["illum_email"].(string)
	illumPass = salty["illum_pword"].(string)
	theShiznit = salty["the_shiznit"].(string)
	statsUrl = salty["ajax_stats"].(string)
}

type DreamService struct {
	gorest.RestService `root:"/api" consumes:"application/json" produces:"application/json"`

	getHistory      gorest.EndPoint `method:"GET" path:"/h/{Name:string}" output:"spicerack.History"`
	getCurrentFight gorest.EndPoint `method:"GET" path:"/f" output:"FightData"`
}

type FightData struct {
	History []spicerack.History
	Stats   spicerack.FighterStats
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

func (serv DreamService) GetCurrentFight() FightData {
	db := spicerack.Db(dbUser, dbPass, dbName)
	defer db.Close()
	fc, err := spicerack.GetSecretData(theShiznit)
	if err != nil {
		serv.ResponseBuilder().SetResponseCode(500)
		return *new(FightData)
	}
	fs, err := spicerack.GetFighterStats(webClient, statsUrl)
	if err != nil {
		serv.ResponseBuilder().SetResponseCode(500)
		return *new(FightData)
	}

	card := &FightData{
		History: make([]spicerack.History, 2),
		Stats:   *fs,
	}

	red, _ := db.GetFighter(fc.RedName)
	blue, _ := db.GetFighter(fc.BlueName)
	card.History[0] = *db.GetHistory(red)
	card.History[1] = *db.GetHistory(blue)

	serv.ResponseBuilder().SetResponseCode(200)
	return *card
}
