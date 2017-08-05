package paste

import (
	"html/template"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/user"
)

func init() {
	http.HandleFunc("/update/", updateHandler)
	http.HandleFunc("/archive/", archiveHandler)
	http.HandleFunc("/", rootHandler)
}

var indexTemplate = template.Must(template.ParseFiles("index.html"))
var pasteTemplate = template.Must(template.ParseFiles("paste.html"))
var archiveTemplate = template.Must(template.ParseFiles("archive.html"))

var pasteIdRegex = regexp.MustCompile(`/([^/]+)`)

type Paste struct {
	// Generated
	Id        string    `datastore:"id"`
	Timestamp time.Time `datastore:"timestamp"`
	// Required
	Content string `datastore:"content,noindex"`
	Email   string `datastore:"email"`
	// Optional/Best-effort
	Title    string `datastore:"title"`
	Language string `datastore:"language"`
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)

	if u == nil {
		c.Infof("%v Login required", appengine.RequestID(c))
		w.WriteHeader(http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var paste Paste
		paste.Id = GenerateRandomString(8)
		paste.Timestamp = time.Now()
		paste.Email = u.Email

		// Pull out title and contents
		r.ParseForm()
		paste.Title = r.Form["title"][0]
		paste.Content = r.Form["contents"][0]

		// Create a key using pasteId and save to datastore
		key := datastore.NewKey(c, "Paste", paste.Id, 0, nil)
		_, err := datastore.Put(c, key, &paste)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Additionally insert in memcache
		item := &memcache.Item{Key: paste.Id, Object: paste}
		memcache.JSON.Set(c, item)

		// Redirect to the newly saved paste
		http.Redirect(w, r, "/"+paste.Id, http.StatusFound)

	case http.MethodDelete:
		pasteId := extractId(r.URL.Path)
		if pasteId == "" {
			http.Error(w, "No paste id found, bad URL", http.StatusBadRequest)
			return
		}

		key := datastore.NewKey(c, "Paste", pasteId, 0, nil)
		var paste Paste
		if err := datastore.Get(c, key, &paste); err != nil {
			c.Errorf("Fetching paste to delete: %v", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Make sure the user performing the delete owns the paste in question
		if paste.Email != u.Email {
			c.Infof("User %v attempting to delete paste %v they do not own", u.Email, paste.Id)
			http.Error(w, "Invalid deletion attempt... only paste owners can delete pastes", http.StatusForbidden)
			return
		}

		if err := datastore.Delete(c, key); err != nil {
			c.Errorf("Deleting paste: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		memcache.Delete(c, pasteId)
	}
}

func archiveHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	q := datastore.NewQuery("Paste").Project("id", "title", "email").Order("-timestamp")
	var pastes []Paste
	_, err := q.GetAll(ctx, &pastes)
	if err != nil {
		ctx.Errorf("Running query: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	archiveTemplate.Execute(w, pastes)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pasteId := extractId(r.URL.Path)
	if pasteId != "" {

		var paste Paste
		// First, lookup in memcache
		_, err := memcache.JSON.Get(c, pasteId, &paste)
		// If there's a miss, check in datastore
		if err == memcache.ErrCacheMiss {
			key := datastore.NewKey(c, "Paste", pasteId, 0, nil)
			err := datastore.Get(c, key, &paste)
			if err != nil {
				http.Error(w, "Paste not found", http.StatusNotFound)
				return
			}
			item := &memcache.Item{Key: pasteId, Object: paste}
			memcache.JSON.Set(c, item)
			c.Infof("Adding %v to memcache", pasteId)
		} else {
			c.Infof("Found %v in memcache", pasteId)
		}

		pasteTemplate.Execute(w, paste)
	} else {
		// show index page
		indexTemplate.Execute(w, nil)
	}
}

func extractId(path string) string {
	match := pasteIdRegex.FindStringSubmatch(path)
	if match == nil {
		return ""
	}
	return match[1]
}

func GenerateRandomString(length int) string {
	// Remove characters that are difficult to disambiguate.
	// Depending on the font various pairs of letters and numbers can be confused
	// - O looks like 0
	// - i looks like l looks like 1 looks like I looks like L
	const chars = "ABCDEFGHJKMNPQRSTUVWXYZ23456789abcdefghjkmnopqrstuvwxyz"
	rand.Seed(time.Now().Unix())
	str := make([]string, length)
	for i := 0; i < length; i++ {
		index := rand.Intn(len(chars))
		str[i] = chars[index : index+1]
	}
	return strings.Join(str, "")
}
