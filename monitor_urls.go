package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"

	"gopkg.in/gomail.v2"
)

// sendmail path
const sendmail = "/usr/sbin/sendmail"

// URLPattern - Struct to hold url, pattern and parameter if we this pattern
// should be, or not found in a page.
type URLPattern struct {
	url     string
	pattern string
	found   bool
}

// URLNotify - should I check that url? Is pattern matched?
type URLNotify struct {
	url    string
	notify bool
}

func getURL(url string) (page string, err error) {
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	// GetURL
	resp, err := client.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		page = string(bodyBytes)
	}
	return
}

func worker(wg *sync.WaitGroup, result chan URLNotify, page URLPattern) {
	defer wg.Done()
	body, err := getURL(page.url)
	if err != nil {
		log.Fatal(err)
	}
	matched, err := regexp.MatchString(page.pattern, body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(page.url + ":" + strconv.FormatBool(matched))
	result <- URLNotify{url: page.url, notify: matched}
}

func submitMail(m *gomail.Message) (err error) {
	cmd := exec.Command(sendmail, "-t")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	pw, err := cmd.StdinPipe()
	if err != nil {
		return
	}

	err = cmd.Start()
	if err != nil {
		return
	}

	var errs [3]error
	_, errs[0] = m.WriteTo(pw)
	errs[1] = pw.Close()
	errs[2] = cmd.Wait()
	for _, err = range errs {
		if err != nil {
			return
		}
	}
	return
}

func main() {

	pages := []URLPattern{}

	// define page and pattern (should be moved to a conf file later)
	p1 := URLPattern{url: "https://apps.fedoraproject.org/nuancier/", pattern: "No elections are currently open for voting", found: false}
	p2 := URLPattern{url: "https://elections.fedoraproject.org/", pattern: "No elections currently open for voting", found: false}
	pages = append(pages, p1, p2)

	// Store results from all workers in a single channel
	results := make(chan URLNotify)

	var wg sync.WaitGroup

	for i := 0; i < len(pages); i++ {
		fmt.Println("Main: Starting worker", i)
		wg.Add(1)
		go worker(&wg, results, pages[i])
	}

	fmt.Println("Main: Waiting for workers to finish")
	go func() {
		wg.Wait()
		close(results)
	}()

	toSend := false
	URLs := ""
	for r := range results {
		if r.notify {
			toSend = true
			URLs += "<b>" + r.url + "</b><br>"
		}
	}

	if toSend {
		m := gomail.NewMessage()
		m.SetHeader("From", "monitor_urls@veryimportant.com")
		m.SetHeader("To", "marcin@szydelscy.pl")
		m.SetHeader("Subject", "Monitored website is in the desired state")

		m.SetBody("text/html", "Check website:<br>"+URLs)
		if err := submitMail(m); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Mail Sent: " + strconv.FormatBool(toSend))
	}
}
