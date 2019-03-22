# An URL shortener written in Golang
Inspired by Mathias Bynens' [PHP URL Shortener](https://github.com/mathiasbynens/php-url-shortener), and triggered by a wish to learn Go, I wanted to try and see if I could build an URL shortener in Go.

## Features

* Redirect to your main website when no slug, or incorrect slug, is entered, e.g. `http://wiere.ma/` → `http://samwierema.nl/`.
* Generates short URLs using only `[a-z0-9]` characters.
* Doesn’t create multiple short URLs when you try to shorten the same URL. In this case, the script will simply return the existing short URL for that long URL.
* Can import old bitly links

## Installation
1. `git clone git@github.com:breyten/go-url-shortener.git`
2. `cd go-url-shortener/docker`
3. Create a config file named `config.(json|yaml|toml)`. Use `config-example.json` as a example.
4. `./run.sh`
5. `docker-compose up -d`
6. Forward requests from your load balancer to the `gus_web_1` container (port 8080)

## To-do
* Add tests
* Add checks for duplicate slugs (i.e. make creation of slugs better)

## Author
* [Breyten Ernsting](http://yerb.net/)
* [Sam Wierema](http://wiere.ma)
