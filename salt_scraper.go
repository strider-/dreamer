package main

/*
	Data scraper for IRC bot; reads fight results from illuminati stat pages &
	stores them in a database.
	TODO:
*/

import (
	"errors"
	"flag"
	"fmt"
	"github.com/moovweb/gokogiri"
	ghtml "github.com/moovweb/gokogiri/html"
	"github.com/moovweb/gokogiri/xml"
	"html"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"spicerack"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	DbUser, DbPass, DbName string
	IllumEmail, IllumPword string
	RecentTournamentCount  int
}

type ParsedMatch struct {
	Red, Blue, Winner          string
	RedBets, BlueBets, Bettors int
	MatchId                    int
	FightWinner                spicerack.FightWinner
}

var (
	repo         *spicerack.Repository
	numRx        *regexp.Regexp
	resetElo     = flag.Bool("reset-elo", false, "Recalcuates elo values")
	eloBase      = flag.Int("elo-base", 300, "Provides a base elo value")
	saltTheEarth = flag.Bool("salt-the-earth", false, "Complete teardown and rebuild.")
)

func main() {
	// load the config file
	flag.Parse()
	conf, err := spicerack.GofigFromEnv("ME_CONF")
	if err != nil {
		fmt.Printf("%v\nQuitting.\n", err)
		os.Exit(1)
	}

	// inflate settings struct & open db connection
	settings := &Settings{}
	conf.Struct("salty", settings)
	repo, err = spicerack.OpenDb(settings.DbUser, settings.DbPass, settings.DbName)
	if err != nil {
		fmt.Printf("Failed to connect to postgres: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	// reset ELO values if options are present
	if *resetElo {
		repo.ResetElo(*eloBase)
	}

	// log into saltybet
	client, err := spicerack.LogIntoSaltyBet(settings.IllumEmail, settings.IllumPword)
	if err != nil {
		fmt.Printf("Error logging into saltybet: %v\n", err)
		os.Exit(1)
	}

	// compile a number regex, we'll be using it a lot in parsing
	numRx, _ = regexp.Compile(`[0-9]+`)

	// scrape the compendium for updated/new characters
	fmt.Println("Scraping Roster")
	if err := getRoster(client); err != nil {
		fmt.Printf("Failed to scrape roster: %v\n", err)
		os.Exit(1)
	}

	// Get the last n number of tournaments & scrape 'em
	count := settings.RecentTournamentCount
	fmt.Printf("Grabbing last %d tournament Ids\n", count)
	var tourneys []int
	if *saltTheEarth {
		tourneys, _ = getAllTournamentIds()
	} else {
		tourneys, err = getLatestTournamentIds(client, count)
		if err != nil {
			fmt.Printf("Failed to grab tournament IDs: %v\n", err)
			os.Exit(1)
		}
	}

	for _, tournyId := range tourneys {
		pageNum := 1
		for {
			fmt.Printf("Processing Tournament #%d, Page #%d\n", tournyId, pageNum)
			hasNextPage, err := processTournament(client, tournyId, pageNum)
			if err != nil {
				fmt.Printf("Failed to parse tournament page: %v\n", err)
				break
			}
			if !hasNextPage {
				break
			}
			fmt.Println()
			pageNum++
		}
		fmt.Println()
	}
	relayToBot(fmt.Sprintf("Scheduled scrape complete, bot information is up to date."))
}

// returns an absolute salty url based on a fragment
func saltyUrl(format string, args ...interface{}) string {
	rel := fmt.Sprintf(format, args...)
	return fmt.Sprintf("http://www.saltybet.com/%s", strings.TrimPrefix(rel, "/"))
}

// checks to ensure we have data to scrape
func illuminatiCheck(rows []xml.Node) (err error) {
	if len(rows) == 0 {
		err = errors.New("unable to find tournaments/fight records, has your illuminati subscription run out?")
	}
	return
}

// grab all characters in the compendium & add/update them.
func getRoster(c *http.Client) error {
	fmt.Printf("- Scraping Compendium\n")
	doc, err := getGokogiriDoc(c, saltyUrl("compendium?search="))
	if err != nil {
		return err
	}
	rows, _ := doc.Search("//ul[@id='tierlist']/li/a")
	for _, r := range rows {
		nums := numRx.FindAllString(r.Attribute("href").String(), 2)
		tier, _ := strconv.Atoi(nums[0])
		cid, _ := strconv.Atoi(nums[1])
		name := nameSub(html.UnescapeString(r.FirstChild().String()))

		fighter, _ := repo.GetFighter(name)

		fighter.CharacterId = cid
		fighter.Name = name
		fighter.Tier = tier
		if err := repo.UpdateFighter(fighter); err != nil {
			fmt.Printf("Failed to update fighter #%d - '%s': %v\n", cid, name, err)
		}
	}
	return nil
}

// For an entire re-scrape, this will be all the valid tournament ids to scrape.
// Rough estimate on first matchmaking fight: Snake Eyes vs Namor; tournament #101, match #51966
func getAllTournamentIds() ([]int, error) {
	return []int{
		101, 102, 103, 104, 105,
		106, 107, 108, 109, 110,
		111, 112, 113, 114, 115,
		116, 117}, nil
}

// Returns an array of the ids of the last n tournaments.
func getLatestTournamentIds(c *http.Client, count int) ([]int, error) {
	doc, err := getGokogiriDoc(c, saltyUrl("stats?tournamentstats=1"))
	if err != nil {
		return nil, err
	}

	rows, _ := doc.Search(fmt.Sprintf("//table/tbody/tr[position() <= %d]", count))
	if err = illuminatiCheck(rows); err != nil {
		return nil, err
	}

	result := make([]int, count)
	for i, r := range rows {
		cols, _ := r.Search("td")
		id, _ := strconv.Atoi(numRx.FindString(cols[0].FirstChild().Attribute("href").String()))
		result[i] = id
	}
	return result, nil
}

// Runs through a tournament page, adding matches & updating fighter information
func processTournament(c *http.Client, id, pageNum int) (bool, error) {
	doc, err := getGokogiriDoc(c, saltyUrl("stats?tournament_id=%d&page=%d", id, pageNum))
	if err != nil {
		return false, err
	}

	rows, _ := doc.Search("//table/tbody/tr")
	if err = illuminatiCheck(rows); err != nil {
		return false, err
	}
	nextpage, _ := doc.Search("//div[@id='pagination']//a[text()='Next']")

	scrapeRows(rows)
	return len(nextpage) > 0, nil
}

// Returns a gokogiri html.Document from a url
func getGokogiriDoc(c *http.Client, url string) (*ghtml.HtmlDocument, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	page, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return gokogiri.ParseHtml(page)
}

// Scrape a match row, parsing information & storing it
func scrapeRows(rows []xml.Node) {
	skipped, updated := 0, 0
	for _, r := range rows {
		pm, err := GetParsedMatch(r)
		if err != nil {
			fmt.Printf("Error parsing match id #%d: %v\n", pm.MatchId, err)
			continue
		}

		if !repo.MatchExists(pm.MatchId) {
			red_fighter, _ := repo.GetFighter(pm.Red)
			blue_fighter, _ := repo.GetFighter(pm.Blue)
			red_fighter.TotalBets += pm.RedBets
			blue_fighter.TotalBets += pm.BlueBets
			spicerack.UpdateFighterElo(red_fighter, blue_fighter, pm.FightWinner)

			if tx, e := repo.StartTransaction(); e == nil {
				if rErr := repo.UpdateFighterInTrans(red_fighter, tx); rErr != nil {
					fmt.Printf("--Skipping match, failed to update fighter: %v\n", rErr)
					tx.Rollback()
				} else if bErr := repo.UpdateFighterInTrans(blue_fighter, tx); bErr != nil {
					fmt.Printf("--Skipping match, failed to update fighter: %v\n", bErr)
					tx.Rollback()
				} else {
					m := &spicerack.Match{
						MatchId: pm.MatchId,
						RedId:   red_fighter.Id, BlueId: blue_fighter.Id,
						RedBets: pm.RedBets, BlueBets: pm.BlueBets,
						BetCount: pm.Bettors, Winner: int(pm.FightWinner),
						Created: time.Now(), Updated: time.Now()}

					if mErr := repo.InsertMatch(m); mErr == nil {
						tx.Commit()
						updated++
					} else {
						tx.Rollback()
						fmt.Printf("--Failed to insert match #%d: %v\n", pm.MatchId, mErr)
					}
				}
			}
		} else {
			skipped++
		}
	}
	fmt.Printf("--Skipped: %d | New Matches: %d\n", skipped, updated)
}

// Parse a match row into a managed object
func GetParsedMatch(n xml.Node) (pm *ParsedMatch, err error) {
	pm = &ParsedMatch{}
	match_url, _ := n.Search("td/a/@href")
	red, _ := n.Search("td/a/span[@class='redtext']/text()")
	redvalue, _ := n.Search("td/a/span[@class='redtext']/following-sibling::text()")
	blue, _ := n.Search("td/a/span[@class='bluetext']/text()")
	bluevalue, _ := n.Search("td/a/span[@class='bluetext']/following-sibling::text()")
	winner, _ := n.Search("td[position() = 2]/span/text()")
	bettors, _ := n.Search("td[last()]/text()")

	if len(match_url) > 0 {
		pm.MatchId, _ = strconv.Atoi(numRx.FindString(match_url[0].String()))
	}
	if len(redvalue) > 0 {
		pm.RedBets, _ = strconv.Atoi(numRx.FindString(redvalue[0].String()))
	}
	if len(bluevalue) > 0 {
		pm.BlueBets, _ = strconv.Atoi(numRx.FindString(bluevalue[0].String()))
	}

	pm.Red = nameSub(html.UnescapeString(red[0].String()))
	pm.Blue = nameSub(html.UnescapeString(blue[0].String()))
	pm.Bettors, _ = strconv.Atoi(bettors[0].String())
	if len(winner) > 0 {
		pm.Winner = nameSub(html.UnescapeString(winner[0].String()))
		if pm.Winner == pm.Red {
			pm.FightWinner = spicerack.WINNER_RED
		} else if pm.Winner == pm.Blue {
			pm.FightWinner = spicerack.WINNER_BLUE
		}
	}

	if pm.MatchId == 0 {
		err = errors.New("Unable to parse match id.")
	} else if len(pm.Red) == 0 || len(pm.Blue) == 0 {
		err = errors.New("Red or Blue fighter is an empty string.")
	} else if int(pm.FightWinner) == 0 {
		err = errors.New("No winner found.")
	} else if pm.MatchId < 51966 {
		err = errors.New("Pre-matchmaking fight. Ignored.")
	}

	return pm, err
}

func nameSub(name string) string {
	// way to go salty, unescaped brackets in html HOW CAN YOU GO WRONG
	if name == " ( 0)/2" {
		return "<> ( 0)<>/2"
	}
	return name
}

// Sends messages to the shaker bot, if listening on this machine at port 4380
func relayToBot(msg string) {
	form := url.Values{
		"Message": {msg},
	}

	host, _ := os.Hostname()
	port := 4380
	url := fmt.Sprintf("http://%s:%d/shaker/bot/talk", host, port)
	if resp, err := http.PostForm(url, form); err != nil {
		fmt.Printf("Failed to send message to IRC bot: %v\n", err)
	} else {
		defer resp.Body.Close()
	}
}
