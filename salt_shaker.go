package main

/*
	IRC bot; reports current fight card w/ stats & hightower link in irc channel
	Commands:
		`wl			 - Reports the page & credentials of detailed win/loss page for current fight card
		`s 		     - Reports the current fight card
		`s  p1 (,p2) - Reports a specific fight card for p1 and/or p2
		`sr p1 (,p2) - Reports a specific fight card for p1 and/or p2 for retired fighters
		`r			 - [Admin] Registers the bot with NickServ
		`c [token]	 - [Admin] Sends a registration confirmation token to NickServ
	TODO:
*/

import (
	"bytes"
	"fmt"
	"github.com/oguzbilgic/socketio"
	"io/ioutil"
	"irc"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"spicerack"
	"strings"
	"syscall"
	"time"
)

const (
	MESSAGE_ENDPOINT string = "/shaker/bot/talk"
	UNKNOWN_FIGHTER  string = "\x02\x0300New Challenger!\x03\x02"
	BOT_ADMIN        string = "Lone_Strider"
	// string formats
	LOG_TIME_FORMAT      string = "2006-01-02 15:04:05"
	P1_NAME_FORMAT       string = "\x02\x0304%s\x03\x02"
	P2_NAME_FORMAT       string = "\x02\x0310%s\x03\x02"
	REMATCH_FORMAT       string = "Rematch! %s has beaten %s!"
	REMATCH_TRADE_FORMAT string = "Rematch! %s and %s have beaten each other!"
	WINNER_FORMAT        string = "%s has defeated %s!"
	UPSET_WINNER_FORMAT  string = "In a %s upset, %s has defeated %s!"
	VS_FORMAT            string = "%s [%s] \x02vs\x02 %s [%s] | %s"
	SOLO_FORMAT          string = "%s [%s] | %s"
	HT_FORMAT            string = "http://fightmoney.herokuapp.com/stats/#/%s/%s"
	TINYURL_FORMAT       string = "http://tinyurl.com/api-create.php?%s"
	WL_MESSAGE           string = "%s [user: %s | pass: %s]"

	UPSET_FACTOR float64 = 2.0
)

type Settings struct {
	DbName, DbUser, DbPass      string
	Server, Channel, Nick, Pass string
	BotEmail                    string
	TheShiznit                  string
	Websocket                   string
	WlAddr, WlUser, WlPass      string
	Pushover                    map[string]interface{}
}

type Options struct {
	LooseSearch   bool
	RetiredSearch bool
}

var (
	settings     *Settings = &Settings{}
	client       *irc.Client
	lastAnnounce time.Time
	shouldNotify bool        = true
	logChannel   chan string = make(chan string)
)

func main() {
	// thread-safe message logging
	go listenForLogs()

	// catching interrupt and terminate signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go catchSignals(sigs)

	// loading from global config
	log("PID: %d\n", os.Getpid())
	log("Loading Configuration")
	conf, err := spicerack.GofigFromEnv("ME_CONF")
	if err != nil {
		log("%v -  Quitting.", err)
		os.Exit(1)
	}

	// reading from config & initialzing IRC client
	conf.Struct("salty", settings)
	settings.Pushover, _ = conf.Map("pushover")
	client = irc.NewClient(settings.Server, settings.Nick, false)

	// If the bot crashes, send a notification
	defer func() {
		if r := recover(); r != nil {
			log("Panic! Sending notification: %v", r)
			msg := fmt.Sprintf("%s is down! (%v)", settings.Nick, r)
			notify(msg)
		}
	}()

	// hookup irc command handling functions
	client.HandleCommand(irc.RPL_WELCOME, registerAndJoin)
	client.HandleCommand(irc.CMD_PRIVMSG, manualFightCard)
	client.HandleCommand(irc.CMD_PRIVMSG, getSpecificFighters)
	client.HandleCommand(irc.CMD_PRIVMSG, getRetiredFighters)
	client.HandleCommand(irc.CMD_PRIVMSG, showWLInfo)
	client.HandleCommand(irc.CMD_PRIVMSG, nickServ)

	// connect to IRC & wait indefinitely, and listen for HTTP posts
	// to relay to the channel
	log("Connecting to IRC...")
	client.Connect()
	listenForRelays()
	client.Wait()

	if shouldNotify {
		notify(fmt.Sprintf("%s has unexpectedly stopped!", settings.Nick))
	}
}

// log messages for debugging
func log(msg string, args ...interface{}) {
	logChannel <- fmt.Sprintf("[%s] %s", time.Now().Format(LOG_TIME_FORMAT), fmt.Sprintf(msg, args...))
}

// go routine for logging messages to stdout
func listenForLogs() {
	for {
		if msg, ok := <-logChannel; ok {
			fmt.Println(strings.TrimRight(msg, "\n"))
		} else {
			break
		}
	}
}

// go routine waiting for an interrupt or terminate signal, so
// the client can be closed gracefully
func catchSignals(sigs <-chan os.Signal) {
	for {
		s := <-sigs
		shouldNotify = false
		log("Recieved OS Signal '%v', closing gracefully.", s)
		client.Quit("Going down for an update, brb")
		break
	}
}

// when connected to the server, identify w/ nickserv, join CHANNEL and start polling salty
func registerAndJoin(m *irc.Message) {
	log("Connected to IRC, registering nick.")
	client.Privmsg("NickServ", fmt.Sprintf("identify %s", settings.Pass))

	log("Joining %s", settings.Channel)
	client.Join(settings.Channel)

	log("Starting websocket loop.")
	go pollSalty()
}

// handles `wl command to announce detailed win/loss page w/ user&pass.
func showWLInfo(m *irc.Message) {
	if m.IsChannelMsg() && m.Parameters[0] == settings.Channel && m.Trail == "`wl" {
		msg := fmt.Sprintf(WL_MESSAGE, settings.WlAddr, settings.WlUser, settings.WlPass)
		client.Privmsg(settings.Channel, msg)
	}
}

// handles `s commands to get the current fight card
func manualFightCard(m *irc.Message) {
	if m.IsChannelMsg() && m.Parameters[0] == settings.Channel && m.Trail == "`s" {
		if data, err := spicerack.GetSecretData(settings.TheShiznit); err == nil {
			announceFightCard(data, nil)
		}
	}
}

// handles `s p1 [, p2]
func getSpecificFighters(m *irc.Message) {
	if m.IsChannelMsg() && m.Parameters[0] == settings.Channel && strings.HasPrefix(m.Trail, "`s ") {
		data := createFightCard(m.Trail[3:])
		opts := &Options{LooseSearch: true}
		announceFightCard(data, opts)
	}
}

// handles `sr p1 [, p2]
func getRetiredFighters(m *irc.Message) {
	if m.IsChannelMsg() && m.Parameters[0] == settings.Channel && strings.HasPrefix(m.Trail, "`sr ") {
		data := createFightCard(m.Trail[4:])
		opts := &Options{RetiredSearch: true}
		announceFightCard(data, opts)
	}
}

// generates a 'fake' fight card for the purposes of reporting specific requested fighters
func createFightCard(str string) *spicerack.FightCard {
	fighters := strings.Split(str, ",")
	fc := &spicerack.FightCard{
		RedName: strings.Trim(fighters[0], " "),
	}

	if len(fighters) > 1 {
		fc.BlueName = strings.Trim(fighters[1], " ")
	}

	return fc
}

// NickServ registration/confirmation
func nickServ(m *irc.Message) {
	if m.Nick == BOT_ADMIN && strings.HasPrefix(m.Trail, "`") {
		switch m.Trail[1:2] {
		case "r":
			client.Privmsg("NickServ", fmt.Sprintf("register %s %s", settings.Pass, settings.BotEmail))
		case "c":
			token := m.Trail[3:]
			client.Privmsg("NickServ", fmt.Sprintf("confirm %s", token))
		}
	} else if !m.IsChannelMsg() {
		// log any private messages the bot gets, why not?
		log("<%s>: %s", m.Nick, m.Trail)
	}
}

// websocket loop
func pollSalty() {
	for {
		socket, err := socketio.DialAndConnect(settings.Websocket, "", "")
		if err != nil {
			log("Failed to connect to websocket: %v. Trying again in 10 sec.", err)
			time.Sleep(time.Second * 10)
			continue
		}

		var lastStatus string = ""
		for {
			_, err := socket.Receive()
			if err != nil {
				log("Failed to receive websocket data: %v. Reconnecting.", err)
				break
			}

			data, err := spicerack.GetSecretData(settings.TheShiznit)
			if err != nil {
				log("%v", err)
			} else {
				if lastStatus != data.Status {
					// reset the cooldown so we always get an up-to-date triggered annoucement
					lastAnnounce = time.Now().Add(time.Second * -5)

					if data.TakingBets() {
						announceFightCard(data, nil)
					} else if data.InProgress() {
						announceOdds(data)
					} else if data.WeHaveAWinner() {
						announceWinner(data)
					}

					lastStatus = data.Status
				}
			}
		}
	}
}

// sends fight card / fighter stats information to IRC
func announceFightCard(data *spicerack.FightCard, opts *Options) {
	if len(data.RedName) == 0 && len(data.BlueName) == 0 {
		return // Nothing to announce!
	}

	diff := time.Now().Sub(lastAnnounce)
	if diff.Seconds() >= 3 {
		db := spicerack.Db(settings.DbUser, settings.DbPass, settings.DbName)
		var red, blue *spicerack.Fighter
		var e error

		if opts != nil && opts.LooseSearch {
			red, blue, e = db.SearchFighters(data.RedName, data.BlueName)
		} else if opts != nil && opts.RetiredSearch {
			red, blue, e = db.SearchRetiredFighters(data.RedName, data.BlueName)
		} else {
			red, blue, e = db.GetFighters(data.RedName, data.BlueName)
		}

		if e == nil {
			p1f := formatFighterName(red, data.RedName, P1_NAME_FORMAT)
			p2f := formatFighterName(blue, data.BlueName, P2_NAME_FORMAT)
			p1stats := formatFighterStats(red)
			p2stats := formatFighterStats(blue)
			ht := getHightowerUrl(data.RedName, data.BlueName)

			var card string = ""
			if len(data.RedName) > 0 && len(data.BlueName) > 0 {
				card = fmt.Sprintf(VS_FORMAT, p1f, p1stats, p2f, p2stats, ht)
			} else if len(data.RedName) > 0 {
				card = fmt.Sprintf(SOLO_FORMAT, p1f, p1stats, ht)
			} else if len(data.BlueName) > 0 {
				card = fmt.Sprintf(SOLO_FORMAT, p2f, p2stats, ht)
			}

			client.Privmsg(settings.Channel, card)

			state, err := db.GetRematchState(red, blue)
			if err == nil {
				switch state {
				case spicerack.TradedWins:
					client.Privmsg(settings.Channel, fmt.Sprintf(REMATCH_TRADE_FORMAT, p1f, p2f))
				case spicerack.RedBeatBlue:
					client.Privmsg(settings.Channel, fmt.Sprintf(REMATCH_FORMAT, p1f, p2f))
				case spicerack.BlueBeatRed:
					client.Privmsg(settings.Channel, fmt.Sprintf(REMATCH_FORMAT, p2f, p1f))
				}
			}

			sprinkleMrsDash(data)
		} else {
			log("%v", e)
		}
		lastAnnounce = time.Now()
	}
}

// sends odds to channel when available
func announceOdds(data *spicerack.FightCard) {
	p1 := formatFighterName(nil, data.RedName, P1_NAME_FORMAT)
	p2 := formatFighterName(nil, data.BlueName, P2_NAME_FORMAT)
	msg := fmt.Sprintf("%s %s %s", p1, data.Odds(), p2)
	client.Privmsg(settings.Channel, msg)
}

// informs channel of the outcome of a match
func announceWinner(data *spicerack.FightCard) {
	p1 := formatFighterName(nil, data.RedName, P1_NAME_FORMAT)
	p2 := formatFighterName(nil, data.BlueName, P2_NAME_FORMAT)
	var w, l, msg string

	if data.Winner() == data.RedName {
		w = p1
		l = p2
	} else if data.Winner() == data.BlueName {
		w = p2
		l = p1
	} else {
		msg = "...I have no idea who won."
	}

	if data.Upset(UPSET_FACTOR) {
		msg = fmt.Sprintf(UPSET_WINNER_FORMAT, data.Odds(), w, l)
	} else {
		msg = fmt.Sprintf(WINNER_FORMAT, w, l)
	}

	client.Privmsg(settings.Channel, msg)
}

// Gives irc formatting to a fighter name, or a fallback name if the fighter isn't in the db.
func formatFighterName(f *spicerack.Fighter, fallback, format string) string {
	if f != nil {
		return fmt.Sprintf(format, f.Name)
	} else {
		return fmt.Sprintf(format, fallback)
	}
}

// Get the stats for a fighter, or the unknown fighter message if the fighter is nil
func formatFighterStats(f *spicerack.Fighter) string {
	if f != nil {
		return f.IrcStats()
	}
	return UNKNOWN_FIGHTER
}

// generates a tinyurl link to hightower for the given fighters
func getHightowerUrl(left, right string) string {
	link := fmt.Sprintf(HT_FORMAT, escapeName(left), escapeName(right))

	v := url.Values{}
	v.Set("url", link)

	turl := fmt.Sprintf(TINYURL_FORMAT, v.Encode())
	if resp, err := http.Get(turl); err == nil {
		defer resp.Body.Close()
		if body, err := ioutil.ReadAll(resp.Body); err == nil {
			return string(body)
		}
	}

	return "TinyURL sucks right now"
}

// escapes fighter names for hightower urls
func escapeName(name string) string {
	result := url.QueryEscape(name)
	return strings.Replace(result, "+", "%20", -1)
}

// listen on an http endpoint for external messages to relay to the channel
func listenForRelays() {
	host, _ := os.Hostname()
	port := 4380
	addr := fmt.Sprintf("%s:%d", host, port)
	http.HandleFunc(MESSAGE_ENDPOINT, handler)
	log("Listening for message relays at http://%s%s", addr, MESSAGE_ENDPOINT)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log("%v", err)
	}
}

// handle any hits to the http endpoint
func handler(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	if r.Method == "POST" {
		msg := r.FormValue("Message")
		if len(msg) > 0 {
			log("Message: %s", msg)
			client.Privmsg(settings.Channel, msg)
			h.Set("X-Success", "true")
		} else {
			h.Set("X-Success", "false")
			h.Set("X-Error", "One or more expected values were missing or incorrect.")
		}
	} else {
		w.WriteHeader(401)
	}
}

// sends pushover notifications
func notify(message string) {
	form := url.Values{
		"user":    {settings.Pushover["user_key"].(string)},
		"token":   {settings.Pushover["irc_token"].(string)},
		"message": {message},
		"sound":   {"siren"}}

	if resp, err := http.PostForm(settings.Pushover["url"].(string), form); err != nil {
		log("Failed to send notification: %v", err)
	} else {
		defer resp.Body.Close()
	}
}

// ...and along comes sexy Mrs. Dash
func sprinkleMrsDash(data *spicerack.FightCard) {
	for _, x := range data.MrsDash {
		switch x {
		case "thats_my_boy":
			client.Privmsg(settings.Channel, rainbowText("ALL IN ON MR. BONEGOLEM'S WILD RIDE"))
		case "fake_astro":
			client.Privmsg(settings.Channel, rainbowText("FAKE ASTRO, DON'T BET"))
		}
	}
}

func rainbowText(text string) string {
	colors := []int{4, 7, 8, 9, 11, 12, 6}
	buf := bytes.NewBuffer(nil)
	skip := 0
	for i, r := range text {
		if r == ' ' {
			skip++
			buf.WriteString(" ")
		} else {
			buf.WriteString(fmt.Sprintf("\x03%02d%s", colors[(i-skip)%(len(colors))], string(r)))
		}
	}
	buf.WriteByte(15)
	return buf.String()
}
