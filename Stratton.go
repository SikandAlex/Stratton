package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

// Helper function to check if a keyword is part of the article keywords
func Find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

// Article stores information about a news article
type Article struct {
	Title       string
	Content     string
	URL         string
	Keywords    []string
	PublishDate *time.Time
}

func main() {
	// Instantiate default collector
	c := colly.NewCollector(

		// Visit only domains: reuters.com, www.reuters.com
		colly.AllowedDomains("reuters.com", "www.reuters.com"),

		// Cache responses to prevent multiple download of pages
		// even if the collector is restarted
		colly.CacheDir("./reuters_cache"),

		colly.Async(true),

		// Set a max depth to prevent too much scraping right now
		//colly.MaxDepth(10),
	)

	// Don't revisit the same URL twice
	c.AllowURLRevisit = false

	// Parallelism
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 100})

	// Create another collector to scrape article detail page
	detailCollector := c.Clone()

	// Create an empty slice that has a capacity of 5,000 Articles
	articles := make([]Article, 0, 5000)

	// On every link with an href attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")

		// If it has '/article' prefix it's an article so have the detailCollector
		// collect the information
		if strings.HasPrefix(link, "/article") {
			// Need to provide the full URL or we get a schema error
			detailCollector.Visit("https://reuters.com" + link)
		}

		// If the link is a next button, we have the original collector handle the paging
		if e.Attr("class") == "control-nav-next" {
			e.Request.Visit(link)
		}

		// Return and exit the callback
		return
	})

	// Before making a request print "Visiting {URL}"
	c.OnRequest(func(r *colly.Request) {
		log.Println("Visiting List Page: ", r.URL.String())
	})

	// Report error back to the console
	c.OnError(func(_ *colly.Response, err error) {
		log.Println("PAGE ERROR:", err)
	})

	detailCollector.OnError(func(_ *colly.Response, err error) {
		log.Println("DETAILS ERROR:", err)
	})

	detailCollector.OnHTML(`html`, func(e *colly.HTMLElement) {

		var keywords []string

		const REUTERS_DATE_FORMAT = "January _2, 2006 3:04 PM"

		var dateSplit = strings.Split(e.ChildText("div.ArticleHeader_date"), "/")

		if len(dateSplit) < 2 {
			return
		}

		var date = strings.TrimSpace(dateSplit[0])
		var timestamp = strings.TrimSpace(dateSplit[1])
		var cleaned = date + " " + timestamp
		var publishedAt, _ = time.Parse(REUTERS_DATE_FORMAT, cleaned)

		fmt.Println(cleaned)
		fmt.Println(publishedAt)
		title := e.ChildText("h1.ArticleHeader_headline")
		body := e.ChildText("div.StandardArticleBody_body")

		e.ForEach("meta[name=keywords]", func(_ int, el *colly.HTMLElement) {
			keywords = strings.Split(el.Attr("content"), ",")
		})

		// If the article isn't about an acquisition then just return from callback
		_, found := Find(keywords, "Mergers / Acquisitions / Takeovers")
		if !found {
			return
		}

		// Construct the article
		a := Article{
			Title:       title,
			Content:     body,
			URL:         e.Request.URL.String(),
			Keywords:    keywords,
			PublishDate: &publishedAt,
		}

		// Append the article to the list of articles
		articles = append(articles, a)
	})

	// Start scraping on the business news section
	c.Visit("https://www.reuters.com/news/archive/businessNews")

	c.Wait()
	detailCollector.Wait()

	// WRITE TO A FILE
	file, _ := json.MarshalIndent(articles, "", "  ")
	_ = ioutil.WriteFile("Acquisitions.json", file, 0644)

	// WRITE TO Stdout
	//enc := json.NewEncoder(os.Stdout)
	//enc.SetIndent("", "  ")
	//enc.Encode(articles)
}
