package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
)

type Link struct {
	Destination string
	Hash        string
}

type LinkAnalytics struct {
	GoTo      *Link
	Analytics []byte
}

func newLink(destination string) *Link {
	// we expect the destination URL to already have been stripped of extra
	//	whitespace by this point
	h := sha256.New()
	h.Write([]byte(destination))

	hash := hex.EncodeToString(h.Sum(nil))
	return &Link{Destination: destination, Hash: hash}
}

func (l *Link) save() error {
	filename := l.Hash + ".linkanalytics"
	contents := []byte(l.Destination + "\n")
	return os.WriteFile(filename, contents, 0600)
}

func loadLink(hash string) (*Link, error) {
	filename := hash + ".linkanalytics"
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// only the first line is the destination
	scanner.Scan()
	destination := scanner.Text()

	return &Link{Destination: destination, Hash: hash}, nil
}

func loadHits(hash string) ([]byte, error) {
	filename := hash + ".linkanalytics"
	hits, err := os.ReadFile(filename)

	if err != nil {
		return nil, err
	}
	return hits, nil
}

func gotHit(hash string, ua string) error {
	filename := hash + ".linkanalytics"
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	logger := log.New(file, "hit: ", log.LstdFlags)
	logger.Println(ua) // add information about the user here later
	defer file.Close()

	return nil
}

var templates = template.Must(template.ParseFiles("create.html", "analytics.html"))

func createHandler(w http.ResponseWriter, r *http.Request, m string) {
	// m is ignored since we're just displaying the form

	// we don't need an actual link since our template never uses it
	err := templates.ExecuteTemplate(w, "create.html", &Link{Destination: "", Hash: ""})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func saveHandler(w http.ResponseWriter, r *http.Request, m string) {
	// m is ignored since we're processing form data from a POST request
	destination := r.FormValue("destination")
	l := newLink(destination)
	err := l.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/analytics/"+l.Hash, http.StatusFound)
}

func analyticsHandler(w http.ResponseWriter, r *http.Request, m string) {
	l, err := loadLink(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h, err2 := loadHits(m)
	if err2 != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a := &LinkAnalytics{l, h}

	err3 := templates.ExecuteTemplate(w, "analytics.html", a)
	if err3 != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func goHandler(w http.ResponseWriter, r *http.Request, m string) {
	l, err := loadLink(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err2 := gotHit(l.Hash, r.Header.Get("User-Agent"))
	if err2 != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, l.Destination, http.StatusFound)
}

func collectHandler(w http.ResponseWriter, r *http.Request, m string) {
	l, err := loadLink(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err2 := gotHit(l.Hash, r.Header.Get("User-Agent"))
	if err2 != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "200 OK %s", m)
}

func validPathComponent(path string) []string {
	validPath := regexp.MustCompile("^/(create|save|analytics|go|collect)/([a-zA-Z0-9]*)$")
	return validPath.FindStringSubmatch(path)
}

// Wraps handlers to remove the boilerplate of checking for valid URLs
func wrapHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPathComponent(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2]) // handlers only need to get what's AFTER their URL component
	}
}

func main() {
	// Contains a form to create a new Link
	//	(this handler does not care about the rest of the URL)
	http.HandleFunc("/create/", wrapHandler(createHandler))

	// Handles form submissions on /create/
	http.HandleFunc("/save/", wrapHandler(saveHandler))

	// Displays analytics for an already-created Link and redirects to /create/
	//	if it doesn't exist yet
	http.HandleFunc("/analytics/", wrapHandler(analyticsHandler))

	// Redirects to the page and collects analytics data
	http.HandleFunc("/go/", wrapHandler(goHandler))

	// Collects analytics data without redirecting
	http.HandleFunc("/collect/", wrapHandler(collectHandler))

	log.Fatal(http.ListenAndServe(":8080", nil))
}
