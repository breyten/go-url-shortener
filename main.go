package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/speps/go-hashids"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"time"
)

var db *sql.DB

func main() {
	// Instantiate the configuration
	viper.SetConfigName("config")
	viper.AddConfigPath("$HOME/.go-url-shortener")
	viper.ReadInConfig()

	// Instantiate the database
	var err error
	dsn := viper.GetString("mysql_user") + ":" + viper.GetString("mysql_password") + "@tcp(" + viper.GetString("mysql_host") + ":3306)/" + viper.GetString("mysql_database") + "?collation=utf8mb4_unicode_ci&parseTime=true"
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	// Instantiate the mux router
	r := mux.NewRouter()
	r.HandleFunc("/s", ShortenHandler).Queries("url", "")
	r.HandleFunc("/{slug:_?[a-zA-Z0-9]+}", ShortenedUrlHandler)
	r.HandleFunc("/", CatchAllHandler)

	// Assign mux as the HTTP handler
	http.Handle("/", r)

	http_address := viper.GetString("http_address")
	if http_address == "" {
		http_address = ":8080" // this is the default address
	}

	log.SetFlags(log.LstdFlags)
	log.Printf("Running on %v", http_address)
	http.ListenAndServe(http_address, nil)
}

// Get the redirect location for a given short url (Ie. 301 or 302 status)
func GetRedirectLocation(short_url string) (error, string) {
	req, err := http.NewRequest("GET", short_url, nil)
	if err != nil {
		return err, ""
	}

	resp, resp_err := http.DefaultTransport.RoundTrip(req)
	if resp_err != nil {
		return resp_err, ""
	}

	defer resp.Body.Close()

	url_obj, loc_err := resp.Location()
	if loc_err != nil {
		return loc_err, ""
	}

	return nil, url_obj.String()
}

// Creates a redirect in the database table
func CreateRedirect(slug string, url string, hits int) error {
	// Insert it into the database
	stmt, err := db.Prepare("INSERT INTO `redirect` (`slug`, `url`, `date`, `hits`) VALUES (?, ?, NOW(), ?)")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(slug, url, hits)
	return err
}

// Generates a slug for a given URL
func GenerateSlug(url string) (string, error) {
	// since time is actually int64 we need to take care of overflows
	const MaxUint = ^uint(0)
	const MinUint = 0
	const MaxInt = int(MaxUint >> 1)
	const MinInt = -MaxInt - 1

	// Get the current time
	var current_time = time.Now()
	var ct_seconds = int(current_time.Unix() % int64(MaxInt))

	// use the technique from http://hashids.org/ to generate a slug
	h := hashids.New()
	s, err := h.Encode([]int{ct_seconds})

	if err != nil {
		return s, err
	}

	// slugs can be prefixed
	viper.SetDefault("slug_prefix", "")
	slug_prefix := viper.GetString("slug_prefix")
	slug := slug_prefix + s

	// now try to insert
	err = CreateRedirect(slug, url, 0)

	// if there was an error it probably means the slug was already used
	if err != nil {
		// so generate a new one by using the nano fraction of unix time
		var ct_nano = int((current_time.UnixNano() - (current_time.Unix() * 1000)) % int64(MaxInt))

		s, err = h.Encode([]int{ct_seconds, ct_nano})
		if err != nil {
			return s, err
		}

		slug = slug_prefix + s

		// now try to insert again
		err = CreateRedirect(slug, url, 0)
	}

	return slug, err
}

// Shortens a given URL passed through in the request.
// If the URL has already been shortened, returns the existing URL.
// Writes the short URL in plain text to w.
func ShortenHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the url parameter has been sent along (and is not empty)
	url := r.URL.Query().Get("url")
	if url == "" {
		log.Printf("ERROR: Bad request, url not included.")
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	// Get the short URL out of the config
	if !viper.IsSet("short_url") {
		log.Printf("ERROR: short url not set")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	short_url := viper.GetString("short_url")

	// Check if url already exists in the database
	var slug string
	err := db.QueryRow("SELECT `slug` FROM `redirect` WHERE `url` = ?", url).Scan(&slug)
	if err == nil {
		log.Printf("/%v -> %v", slug, url)
		// The URL already exists! Return the shortened URL.
		w.Write([]byte(short_url + "/" + slug))
		return
	}

	// It doesn't exist! Generate a new slug for it
	slug, err = GenerateSlug(url)

	if err != nil {
		log.Printf("ERROR: %v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("/%v created", slug)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(short_url + "/" + slug))
}

// Handles a requested short URL.
// Redirects with a 301 header if found.
func ShortenedUrlHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Check if a slug exists
	vars := mux.Vars(r)
	slug, ok := vars["slug"]
	if !ok {
		log.Printf("ERROR: Bad request")
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	// 2. Check if the slug exists in the database
	var url string
	err := db.QueryRow("SELECT `url` FROM `redirect` WHERE `slug` = ?", slug).Scan(&url)
	if err != nil {
		// 2a. Check for a fallback url. This is a string for formatting
		//     This can be something like: http://bit.ly/%s
		//     But if there is no fallback we will return a not found error
		if !viper.IsSet("fallback_url") {
			log.Printf("WARN: /%v not found", slug)
			http.NotFound(w, r)
			return
		} else {
			// get the redirect location
			err, url = GetRedirectLocation(fmt.Sprintf(viper.GetString("fallback_url"), slug))
			if err != nil {
				log.Printf("ERROR: %v", err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Insert it into the database
			err = CreateRedirect(slug, url, 0)
			if err != nil {
				log.Printf("ERROR: %v", err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {
				log.Printf("/%v created", slug)
			}
		}
	}

	// 3. If the slug (and thus the URL) exist, update the hit counter
	stmt, err := db.Prepare("UPDATE `redirect` SET `hits` = `hits` + 1 WHERE `slug` = ?")
	if err != nil {
		log.Printf("ERROR: %v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = stmt.Exec(slug)
	if err != nil {
		log.Printf("ERROR: %v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Finally, redirect the user to the URL
	log.Printf("%v -> %v", r.URL, url)
	http.Redirect(w, r, url, http.StatusMovedPermanently)
}

// Catches all other requests to the short URL domain.
// If a default URL exists in the config redirect to it.
func CatchAllHandler(w http.ResponseWriter, r *http.Request) {

	// 1. Get the redirect URL out of the config
	if !viper.IsSet("default_url") {
		// The reason for using StatusNotFound here instead of StatusInternalServerError
		// is because this is a catch-all function. You could come here via various
		// ways, so showing a StatusNotFound is friendlier than saying there's an
		// error (i.e. the configuration is missing)
		log.Printf("WARN: Catch all requested but default_url not set")
		http.NotFound(w, r)
		return
	}

	// 2. If it exists, redirect the user to it
	default_url := viper.GetString("default_url")
	log.Printf("%v -> %v", r.URL, default_url)
	http.Redirect(w, r, default_url, http.StatusMovedPermanently)
}
