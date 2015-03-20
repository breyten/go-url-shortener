package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"fmt"
	"log"
	"math/rand"
	"net/http"
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
	r.HandleFunc("/{slug:[a-zA-Z0-9]+}", ShortenedUrlHandler)
	r.HandleFunc("/", CatchAllHandler)

	// Assign mux as the HTTP handler
	http.Handle("/", r)

	http_address := viper.GetString("http_address")
	if http_address == "" {
		http_address = ":8080" // this is the default address
	}

	http.ListenAndServe(http_address, nil)
}

// Creates a redirect in the database table
func CreateRedirect(slug string, url string, hits int) (error) {
	// Insert it into the database
	stmt, err := db.Prepare("INSERT INTO `redirect` (`slug`, `url`, `date`, `hits`) VALUES (?, ?, NOW(), ?)")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(slug, url, hits)
	return err
}


// Shortens a given URL passed through in the request.
// If the URL has already been shortened, returns the existing URL.
// Writes the short URL in plain text to w.
func ShortenHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the url parameter has been sent along (and is not empty)
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	// Get the short URL out of the config
	if !viper.IsSet("short_url") {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	short_url := viper.GetString("short_url")

	// Check if url already exists in the database
	var slug string
	err := db.QueryRow("SELECT `slug` FROM `redirect` WHERE `url` = ?", url).Scan(&slug)
	if err == nil {
		// The URL already exists! Return the shortened URL.
		w.Write([]byte(short_url + "/" + slug))
		return
	}

	// It doesn't exist! Generate a new slug for it
	// From: http://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-golang
	var chars = []rune("0123456789abcdefghijklmnopqrstuvwxyz")
	s := make([]rune, 6)
	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}

	slug = string(s)

	// Insert it into the database
	err = CreateRedirect(slug, url, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
			http.NotFound(w, r)
			return
		} else {
			req, err := http.NewRequest("GET", fmt.Sprintf(viper.GetString("fallback_url"), slug), nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			resp, resp_err := http.DefaultTransport.RoundTrip(req)
			if resp_err != nil {
				http.Error(w, resp_err.Error(), http.StatusInternalServerError)
				return
			}

			defer resp.Body.Close()

			url_obj, loc_err := resp.Location()
			if loc_err != nil {
				http.Error(w, loc_err.Error(), http.StatusInternalServerError)
				return
			}
			url = url_obj.String()
			// Insert it into the database
			err = CreateRedirect(slug, url, 0)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
    }
	}

	// 3. If the slug (and thus the URL) exist, update the hit counter
	stmt, err := db.Prepare("UPDATE `redirect` SET `hits` = `hits` + 1 WHERE `slug` = ?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = stmt.Exec(slug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Finally, redirect the user to the URL
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
		http.NotFound(w, r)
		return
	}

	// 2. If it exists, redirect the user to it
	http.Redirect(w, r, viper.GetString("default_url"), http.StatusMovedPermanently)
}
