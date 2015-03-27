package hello

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/user"

	"github.com/dchest/uniuri"
	"github.com/zenazn/goji/web"
)

type Link struct {
	Key    string    `json:"key"`
	Author string    `json:"author"`
	Slug   string    `json:"slug"`
	Target string    `json:"target"`
	Date   time.Time `json:"date"`
}

var DOMAIN = "example.com"

func GetUser(ctx appengine.Context) *user.User {
  user := user.Current(ctx)
  if user == nil {
    return nil
  }
  if !strings.HasSuffix(user.Email, "@" + DOMAIN) {
    return nil
  }
  return user
}

func GetLinkRedirect(c web.C, w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	
	q := datastore.NewQuery("Link").Filter("Slug =", c.URLParams["slug"])
  link := Link{}
	key, err := q.Run(ctx).Next(&link)
	if err == datastore.Done {
   	http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Add("X-Key", key.Encode())
	http.Redirect(w, r, link.Target, http.StatusFound)
}

func GetLink(c web.C, w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	key, err := datastore.DecodeKey(c.URLParams["key"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	l := Link{}
	err = datastore.Get(ctx, key, &l)
	if err == datastore.ErrNoSuchEntity {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	l.Key = key.Encode()
	json.NewEncoder(w).Encode(l)
}

func PutLink(c web.C, w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	u := GetUser(ctx)
	if u == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	l := Link{}
	err := json.NewDecoder(r.Body).Decode(&l)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	key, err := datastore.DecodeKey(c.URLParams["key"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	l.Key = "" // key is not part of the data we store
	_, err = datastore.Put(ctx, key, &l)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func DeleteLink(c web.C, w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	if GetUser(ctx) == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	key, err := datastore.DecodeKey(c.URLParams["key"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = datastore.Delete(ctx, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func GetAllLinks(c web.C, w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	if GetUser(ctx) == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	links := []Link{}
	iterator := datastore.NewQuery("Link").Run(ctx)
	for {
		var link Link
		key, err := iterator.Next(&link)
		if err == datastore.Done {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		link.Key = key.Encode()
		links = append(links, link)
	}
	json.NewEncoder(w).Encode(links)
}

func AddLink(c web.C, w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	remoteUser := GetUser(ctx)
	if remoteUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	l := Link{
		Slug:   r.FormValue("slug"),
		Target: r.FormValue("target"),
		Date:   time.Now(),
		Author: remoteUser.Email,
	}
	if strings.HasPrefix(r.Header.Get("Content-type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if l.Slug == "" {
		l.Slug = uniuri.New()
	}

  key := datastore.NewIncompleteKey(ctx, "Link", nil)
	_, err := datastore.Put(ctx, key, &l)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	l.Key = key.Encode()
	json.NewEncoder(w).Encode(l)
}

func GetRoot(c web.C, w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	if GetUser(ctx) == nil {
	  if user.Current(ctx) != nil {
 		  http.Error(w, "Unauthorized", http.StatusUnauthorized)
		  return
	  }
		url, err := user.LoginURL(ctx, r.URL.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", url)
		w.WriteHeader(http.StatusFound)
		return
	}
	http.ServeFile(w, r, "index.html")
}

func init() {
	mux := web.New()
	mux.Get("/links/", GetAllLinks)
	mux.Post("/links/", AddLink)
	mux.Get("/links/:key", GetLink)
	mux.Put("/links/:key", PutLink)
	mux.Delete("/links/:key", DeleteLink)
	mux.Get("/", GetRoot)
	mux.Get("/:slug", GetLinkRedirect)

	http.Handle("/", mux)
	http.Handle("/*", mux)
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, world!")
}
