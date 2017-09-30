package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
)

type Episode struct {
	Title       string
	Description string
	Url         string
}

type Resource struct {
	Url  string
	Name string
}

type AuthenticationError struct {
	s string
}

func (e *AuthenticationError) Error() string {
	return e.s
}

type Dpd struct {
	Name        string
	client      *http.Client
	htmlCatalog *goquery.Document
}

func (d *Dpd) Login() bool {
	d.initClient()

	resp, _ := d.client.PostForm(
		d.loginUrl(),
		url.Values{
			"username": {os.Getenv("PDP_USER")},
			"password": {os.Getenv("PDP_PASS")},
		})
	doc, _ := goquery.NewDocumentFromResponse(resp)
	failureMessage, _ := doc.Find("#cart-body > .notice").Html()
	if failureMessage == "" {
		d.htmlCatalog = doc
		return true
	} else {
		return false
	}
}

func (d *Dpd) Episodes() []Episode {
	var episodes []Episode

	d.htmlCatalog.Find(".blog-entry").Each(func(_ int, s *goquery.Selection) {
		episode := d.episodeFromSelection(s)
		episodes = append(episodes, episode)
	})

	return episodes
}

func (d *Dpd) ResourcesForEpisode(episode Episode) []Resource {
	var resources []Resource
	anchors := d.fetchPage(episode.Url).Find(".blog-entry > ul").Find("li > a")

	for i := range anchors.Nodes {
		href, _ := anchors.Eq(i).Attr("href")
		name := anchors.Eq(i).Text()
		url := fmt.Sprint(d.site(), href)
		resources = append(resources, Resource{Url: url, Name: name})
	}

	return resources
}

func (d *Dpd) SaveResource(resource Resource, directory string) bool {
	filepath := filepath.Join(".", directory, resource.Name)
	out, err := os.Create(filepath)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer out.Close()

	resp, err := d.client.Get(resource.Url)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println(err)
		return false
	}

	return true
}

// private functions
func (d *Dpd) episodeFromSelection(page *goquery.Selection) Episode {
	title := page.Find("h3").Text()
	description := page.Find(".blog-content").Text()
	path, _ := page.Find(".content-post-meta span a").Attr("href")
	url := d.urlFor(path)

	return Episode{Title: title, Description: description, Url: url}
}

func (d *Dpd) fetchPage(url string) *goquery.Document {
	response, _ := d.client.Get(url)
	doc, _ := goquery.NewDocumentFromResponse(response)
	return doc
}

func (d *Dpd) loginUrl() string {
	homeUrl := d.urlFor("/subscriber/content")
	form := d.fetchPage(homeUrl).Find(".cart-form")
	loginAction, _ := form.Attr("action")
	return d.urlFor(loginAction)
}

func (d *Dpd) urlFor(path string) string {
	return fmt.Sprint(d.site(), path)
}

func (d *Dpd) initClient() {
	gCurCookieJar, _ := cookiejar.New(nil)
	d.client = &http.Client{
		CheckRedirect: nil,
		Jar:           gCurCookieJar,
	}
}

func (d *Dpd) site() string {
	return fmt.Sprint("https://", d.Name, ".dpdcart.com")
}

// ------- main ------
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Argument required for subdomain to crawl")
		return
	}

	dpd := Dpd{Name: os.Args[1]}
	if dpd.Login() == false {
		fmt.Println("Invalid username and password combination. Make sure to set DPD_USER and DPD_PASS environment variables.")
		return
	}

	for _, episode := range dpd.Episodes() {
		fmt.Println("Downloading episode: ", episode.Title)
		downloadPath := filepath.Join(".", "downloads", episode.Title)
		os.MkdirAll(downloadPath, os.ModePerm)

		for _, resource := range dpd.ResourcesForEpisode(episode) {
			if dpd.SaveResource(resource, downloadPath) == false {
				fmt.Println("Unable to save resource")
				return
			}
		}
	}
}
