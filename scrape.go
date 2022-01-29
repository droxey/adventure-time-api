package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	cmap "github.com/orcaman/concurrent-map"

	. "github.com/logrusorgru/aurora"
)

const (
	visitExternalLinks  = false
	debugging           = true
	fileName            = "results.json"
	minValidity         = 2
	minStrainNameLength = 2
	threads             = 8
	maxDepth            = 3
	filePermissions     = 0644
	runAsync            = true
	baseURL             = "https://adventuretime.fandom.com"
	firstPage           = "https://adventuretime.fandom.com/wiki/Category:Transcripts"
	secondPage          = "https://adventuretime.fandom.com/wiki/Category:Transcripts?from=Seventeen%2FTranscript"
)

func main() {
	startTime := time.Now()
	episodeURLs := make([]string, 0)
	characterLineMap := cmap.New()
	c := setupCollector()
	// q, _ := queue.New(threads, &queue.InMemoryQueueStorage{MaxSize: 10000})

	c.OnHTML("#content .category-page__member-link", func(e *colly.HTMLElement) {
		scriptLink, linkExists := e.DOM.Attr("href")
		title := e.DOM.Text()
		isDirectoryPage := strings.Contains(strings.ToLower(title), "transcripts")

		if linkExists && !isDirectoryPage {
			episodeURLs = append(episodeURLs, baseURL+scriptLink)
			c.Visit(baseURL + scriptLink)
		}
	})

	c.OnHTML("#mw-content-text > div > dl > dd", func(e *colly.HTMLElement) {
		if e.DOM.Find("b").Text() != "" {
			character := strings.TrimSpace(strings.ToLower(e.DOM.Find("b").Text()))
			characterAndLine := strings.Split(e.Text, character)

			if len(character) >= 3 && characterAndLine != nil && len(characterAndLine) > 1 {
				if !characterLineMap.Has(character) {
					characterLineMap.Set(character, make([]string, 0))
				}

				if len(characterAndLine[1]) > 5 {
					line := strings.TrimSpace(strings.ReplaceAll(characterAndLine[1], ":", ""))
					cleanLine := line //getActionTextFromDialogue(line)
					if tmp, ok := characterLineMap.Get(character); ok {
						characterLines := tmp.([]string)
						characterLines = append(characterLines, cleanLine)
						characterLineMap.Set(character, characterLines)
					}
				}
			}
		}
	})

	fmt.Println(Bold("\nStarting scan...\n"))

	c.Visit(firstPage)
	c.Wait()

	c.Visit(secondPage)
	c.Wait()

	output, _ := json.MarshalIndent(characterLineMap, "", "  ")
	ioutil.WriteFile(fileName, output, filePermissions)

	fmt.Println(
		Sprintf("%s %d episodes found in %2.2f minutes and saved to %s.",
			Inverse("[DONE]").Bold(),
			Green(len(episodeURLs)).Bold(),
			Green(time.Now().Sub(startTime).Minutes()).Bold(),
			Blue(fileName).Bold()))

	os.Exit(1)

}

func setupCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.MaxDepth(maxDepth),
		colly.Async(runAsync),
		colly.CacheDir("./.cache"),
	)

	extensions.RandomUserAgent(c)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: threads,
		RandomDelay: 5 * time.Second})

	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
		DialContext: (&net.Dialer{
			Timeout: 20 * time.Second,
		}).DialContext,
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", RandomString())

		if debugging {
			fmt.Println(Sprintf("%s %s", Green("[REQ]"), r.URL.String()))
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		if debugging {
			fmt.Println(Sprintf("%s %d %s: %s", Red("[ERR]"), Red(r.StatusCode).Bold(), Red(err.Error()).Bold(), r.Request.URL.String()))
		}
	})

	return c
}

func log(msg string) {
	fmt.Println(Sprintf("%s %s", White("[DEBUG]").BgGray(8-1), Blue(msg)))
}

func getActionTextFromDialogue(line string) string {
	re := regexp.MustCompile(`\[([^\[\]]*)\]`)
	submatchall := re.FindAllString(line, -1)

	for _, element := range submatchall {
		line = strings.TrimSpace(strings.Replace(strings.Replace(strings.Replace(strings.Replace(line, element, "", 1), "]", "", 1), "[", "", 1), ".", "", 1))
	}

	return line
}
