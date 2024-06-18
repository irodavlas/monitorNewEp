package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/vinted/data"
)

const (
	UK_CURRENCY = "£"
	FR_CURRENCY = "€"
	DE_CURRENCY = "€"
	PL_CURRENCY = "zł"
)
const (
	UK_WEBHOOK = "https://discordapp.com/api/webhooks/1252001796361162843/5bzY987n-6-hibjcAtmJbPfZoICSfGbV6TDT_XUn_Xr7izhYxBatv4tn3RHuFxczRByZ"
	FR_WEBHOOK = "https://discordapp.com/api/webhooks/1252276637320609803/UO43qxvtq-zJvhgjWIcE5sh8rYZNcyB8cEC1n1SHT5o8QEqA2F64gJQShg-bM-Eu3cAF"
	DE_WEBHOOK = "https://discordapp.com/api/webhooks/1252276571931410522/elu93W1O7uqYY3kaQzaGT4xcgZhT9SPgmffjAoH_r8dlXFGNnKGfWRXD0T35CNT8klgN"
	PL_WEBHOOK = "https://discordapp.com/api/webhooks/1252276519913525400/erbkXOsWU0QQoUqME_gbGepgqJMi5ZZywR-TcokbTUJSsEOPN5QuiT4D7JQVy1eRt1w7"
)
const (
	UK_BASE_URL = "https://www.vinted.co.uk"
	FR_BASE_URL = "https://www.vinted.fr"
	DE_BASE_URL = "https://www.vinted.de"
	PL_BASE_URL = "https://www.vinted.pl"
)

var session_channel chan string
var error_channel chan string
var product_channel chan data.Item

var proxies []string

type Links struct {
	UK_links []string
	FR_links []string
	DE_links []string
	PL_links []string
}
type Monitor struct {
	Client       Client
	FOUND_SKU    []int
	UrlToMonitor string
}
type Client struct {
	TlsClient *tls_client.HttpClient
	url       string
	Region    string
}

type Options struct {
	settings []tls_client.HttpClientOption
}

func NewClient(_url string, region string) (*Client, error) {
	options := Options{
		settings: []tls_client.HttpClientOption{
			tls_client.WithTimeoutSeconds(15),
			tls_client.WithClientProfile(profiles.Chrome_124),
			tls_client.WithNotFollowRedirects(),
		},
	}
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options.settings...)
	if err != nil {
		return nil, err
	}

	return &Client{TlsClient: &client, url: _url, Region: region}, nil
}
func NewMonitor(link string, clientUrl string) (*Monitor, error) {
	client, err := NewClient(clientUrl, clientUrl[len(clientUrl)-2:])

	if err != nil {
		return nil, err
	}
	monitor := Monitor{
		Client:       *client,
		FOUND_SKU:    []int{},
		UrlToMonitor: link,
	}
	return &monitor, nil
}
func main() {
	var monitorsUK []Monitor
	var monitorsFR []Monitor
	var monitorsDE []Monitor
	var monitorsPL []Monitor
	error_channel = make(chan string)
	session_channel = make(chan string)
	product_channel = make(chan data.Item)

	links := &Links{}

	parse_url_file(links)
	parse_proxy_file()

	//monitor def

	//Uk monitors
	for _, url := range links.UK_links {
		monitor, err := NewMonitor(url, UK_BASE_URL)
		if err != nil {
			log.Println("Error creating monitor for uk: " + err.Error())
		}
		monitorsUK = append(monitorsUK, *monitor)
	}
	for _, url := range links.FR_links {
		monitor, err := NewMonitor(url, FR_BASE_URL)
		if err != nil {
			log.Println("Error creating monitor for fr: " + err.Error())
		}
		monitorsFR = append(monitorsFR, *monitor)
	}
	for _, url := range links.DE_links {
		monitor, err := NewMonitor(url, DE_BASE_URL)
		if err != nil {
			log.Println("Error creating monitor for de: " + err.Error())
		}
		monitorsDE = append(monitorsDE, *monitor)
	}
	for _, url := range links.PL_links {
		monitor, err := NewMonitor(url, PL_BASE_URL)
		if err != nil {
			log.Println("Error creating monitor for pl: " + err.Error())
		}
		monitorsPL = append(monitorsPL, *monitor)
	}

	session_client, err := NewClient("https://www.vinted.com", "All")
	if err != nil {
		log.Fatal("Error creating client to fetch session")
	}

	go get_session(session_client)
	for _, monitor := range monitorsUK {
		go monitor.new_product_monitor(&monitor.Client)
	}
	for _, monitor := range monitorsFR {
		go monitor.new_product_monitor(&monitor.Client)
	}
	for _, monitor := range monitorsDE {
		go monitor.new_product_monitor(&monitor.Client)
	}
	for _, monitor := range monitorsPL {
		go monitor.new_product_monitor(&monitor.Client)
	}

	//go read_prods()
	/*
		session_client, err := NewClient("https://www.vinted.com", "All")
		if err != nil {
			log.Fatal("Error creating client to fetch session")
		}



			go get_session(session_client)
			go monitors[0].new_product_monitor(&monitors[0].Client)
	*/
	go func() {
		for {
			select {
			case errMsg := <-error_channel:
				fmt.Println("Error:", errMsg)
			case session := <-session_channel:
				if session != "" {
					fmt.Println("Session:", session[1:10])
				}
			case prod := <-product_channel:

				if prod.Region == "uk" {
					send_webhook(UK_WEBHOOK, UK_CURRENCY, prod)
				}
				if prod.Region == "fr" {
					send_webhook(FR_WEBHOOK, FR_CURRENCY, prod)

				}
				if prod.Region == "de" {
					send_webhook(DE_WEBHOOK, DE_CURRENCY, prod)

				}
				if prod.Region == "pl" {
					send_webhook(PL_WEBHOOK, PL_CURRENCY, prod)

				}
			}

		}
	}()
	// Keep the main function running
	select {}

}
func formatProxy(proxy string) string {
	// Split the proxy string by colon
	parts := strings.Split(proxy, ":")

	// Check if the proxy has the correct number of parts
	if len(parts) != 4 {
		return "Invalid proxy format"
	}

	// Extract host, port, user, and pass
	host := parts[0]
	port := parts[1]
	user := parts[2]
	pass := parts[3]

	// Format the proxy in the required format
	formattedProxy := fmt.Sprintf("http://%s:%s@%s:%s", user, pass, host, port)
	return formattedProxy
}
func rotate_proxy(client tls_client.HttpClient) {

	randomInt := rand.Intn(len(proxies))
	err := client.SetProxy(proxies[randomInt])
	if err != nil {
		error_channel <- "Error changing proxies: " + err.Error()
	}

}
func parse_proxy_file() {
	file, err := os.Open("proxy.txt")
	if err != nil {
		error_channel <- err.Error()
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		proxy := formatProxy(line)
		proxies = append(proxies, proxy)
	}
}

// add other regions
func parse_url_file(links *Links) {
	file, err := os.Open("links.txt")
	if err != nil {
		error_channel <- err.Error()
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		new := strings.Split(line, "?")
		if strings.Contains(line, "https://www.vinted.co.uk") {

			url := UK_BASE_URL + "/api/v2/catalog/items?" + new[1]
			links.UK_links = append(links.UK_links, url)
		}
		if strings.Contains(line, "https://www.vinted.fr") {

			url := FR_BASE_URL + "/api/v2/catalog/items?" + new[1]
			links.FR_links = append(links.FR_links, url)
		}
		if strings.Contains(line, "https://www.vinted.de") {

			url := DE_BASE_URL + "/api/v2/catalog/items?" + new[1]
			links.DE_links = append(links.DE_links, url)
		}
		if strings.Contains(line, "https://www.vinted.pl") {
			url := PL_BASE_URL + "/api/v2/catalog/items?" + new[1]
			links.PL_links = append(links.PL_links, url)
		}

	}
	defer file.Close()
}
func (m *Monitor) new_product_monitor(client *Client) {

	for {
		session := <-session_channel

		url := m.UrlToMonitor
		rotate_proxy(*client.TlsClient)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			error_channel <- "[" + m.Client.Region + "]" + "Retrying, Error occured: " + err.Error()
			time.Sleep(2 * time.Second)
			continue
		}

		req.Header = http.Header{}

		req.Header.Add("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		req.Header.Add("accept-language", "en-US,en;q=0.9")
		req.Header.Add("cookie", fmt.Sprintf("_vinted_fr_session=%s", session))
		req.Header.Add("sec-ch-ua", "\"Google Chrome\";v=\"125\", \"Chromium\";v=\"125\", \"Not.A/Brand\";v=\"24\"")
		req.Header.Add("sec-ch-ua-mobile", "?0")
		req.Header.Add("sec-ch-ua-platform", "\"Windows\"")
		req.Header.Add("upgrade-insecure-requests", "1")
		req.Header.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")

		resp, err := (*client.TlsClient).Do(req)
		if err != nil {
			error_channel <- "[" + m.Client.Region + "]" + "Retrying, Error occured while firing the request: " + err.Error()
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.StatusCode == 200 {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				error_channel <- "[" + m.Client.Region + "]" + "Retrying, Error occured opening body: " + err.Error()
				resp.Body.Close()
				time.Sleep(2 * time.Second)
				continue
			}
			var catalog data.CatalogueItems
			err = json.Unmarshal(body, &catalog)

			if err != nil {
				error_channel <- "[" + m.Client.Region + "]" + "Retrying, Error occured while unmarshaling: " + err.Error()
				return
			}
			log.Println("[" + m.Client.Region + "]" + "Succesfully made request to products page")

			if len(m.FOUND_SKU) <= 0 {
				for _, item := range catalog.Items {

					m.FOUND_SKU = append(m.FOUND_SKU, int(item.ID))

				}
				log.Println("[" + m.Client.Region + "]" + "First iteration finished")
				continue
			}
			for _, item := range catalog.Items {
				if !slices.Contains(m.FOUND_SKU, int(item.ID)) {

					//append to found skus
					m.FOUND_SKU = append(m.FOUND_SKU, int(item.ID))

					//set few personalized informations
					item.Region = m.Client.Region
					DropTime := time.Now()

					item.Timestamp = DropTime
					go get_product_timestamp(*client.TlsClient, client.url+"/api/v2/items/", item.ID, session, item)

					log.Println("["+m.Client.Region+"]"+"Found New Item with SKU: ", item.ID)
				}
			}

		} else {
			error_channel <- fmt.Sprintf("["+m.Client.Region+"]"+"Status Code [%d]", resp.StatusCode)
			time.Sleep(2 * time.Second)

			continue
		}

	}

}
func get_product_timestamp(client tls_client.HttpClient, api_Url string, Sku int64, session string, item data.Item) {
	retries := -1
	for {
		retries += 1
		rotate_proxy(client)

		if retries == 5 {
			//log.Println("Max retry limit hit, SKU is now no longer being monitored")
			error_channel <- "Max retry limit hit, " + strconv.FormatInt(Sku, 10) + ". is no longer being monitored"
			return
		}
		url := api_Url + strconv.FormatInt(Sku, 10)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			if strings.Contains(err.Error(), "cannot assign requested address") {
				error_channel <- "Error occured: [ERR500] " + err.Error()
				return

			}
			error_channel <- "retrying on Sku: " + strconv.FormatInt(Sku, 10) + " error: " + err.Error()

			continue
		}
		req.Header = http.Header{}
		req.Header.Add("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		req.Header.Add("accept-language", "en-US,en;q=0.9")
		req.Header.Add("cache-control", "no-cache")
		req.Header.Add("cookie", fmt.Sprintf("_vinted_fr_session=%s", session))
		req.Header.Add("pragma", "no-cache")
		req.Header.Add("priority", "u=0, i")
		req.Header.Add("sec-ch-ua", "\"Google Chrome\";v=\"125\", \"Chromium\";v=\"125\", \"Not.A/Brand\";v=\"24\"")
		req.Header.Add("sec-ch-ua-mobile", "?0")
		req.Header.Add("sec-ch-ua-platform", "\"Windows\"")
		req.Header.Add("sec-fetch-dest", "document")
		req.Header.Add("sec-fetch-mode", "navigate")
		req.Header.Add("sec-fetch-site", "none")
		req.Header.Add("sec-fetch-user", "?1")
		req.Header.Add("upgrade-insecure-requests", "1")
		req.Header.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")

		resp, err := (client).Do(req)
		if err != nil {
			error_channel <- "Retrying, Error occured on Sku: " + strconv.FormatInt(Sku, 10) + " Error: " + err.Error()
			continue
		}
		if resp.StatusCode != 200 {
			error_channel <- "Retrying, Error occured on Sku: " + strconv.FormatInt(Sku, 10) + " Error: " + fmt.Sprintf("Status code: %d", resp.StatusCode)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			error_channel <- "Retrying, Error occured on Sku: " + strconv.FormatInt(Sku, 10) + " Error: " + err.Error()
			resp.Body.Close()
			continue
		}
		/*
			var data data.ItemDetails
			err = json.Unmarshal(body, &data)
			if err != nil {
				error_channel <- "Retrying, Error occured on Sku: " + strconv.FormatInt(Sku, 10) + " Error: " + err.Error()
				continue
			}
			fmt.Println("Item was created at:", data.ID)
		*/
		var bodyMap map[string]interface{}

		// Unmarshal JSON into the map
		err = json.Unmarshal(body, &bodyMap)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}
		createdAtTs, ok := bodyMap["item"].(map[string]interface{})["created_at_ts"].(string)
		if !ok {
			error_channel <- "created_at_ts field not found or not of type string"
			continue
		}

		createdAt, err := time.Parse(time.RFC3339, createdAtTs)
		if err != nil {
			error_channel <- "Error parsing created_at_ts:" + err.Error()
			continue
		}

		diff := item.Timestamp.Sub(createdAt)
		item.TimeDiff = diff

		product_channel <- item
		log.Println("item sent closing goroutine")
		return

	}

}
func get_session(client *Client) {
	for {

		req, err := http.NewRequest(http.MethodHead, client.url, nil)
		if err != nil {
			error_channel <- "Error fetching session with session client:" + err.Error()
			session_channel <- ""
			continue
		}

		req.Header = http.Header{}
		req.Header.Add("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		req.Header.Add("accept-language", "en-US,en;q=0.9")
		req.Header.Add("cache-control", "no-cache")
		req.Header.Add("pragma", "no-cache")
		req.Header.Add("priority", "u=0, i")
		req.Header.Add("sec-ch-ua", "\"Google Chrome\";v=\"125\", \"Chromium\";v=\"125\", \"Not.A/Brand\";v=\"24\"")
		req.Header.Add("sec-ch-ua-mobile", "?0")
		req.Header.Add("sec-ch-ua-platform", "\"Windows\"")
		req.Header.Add("sec-fetch-dest", "document")
		req.Header.Add("sec-fetch-mode", "navigate")
		req.Header.Add("sec-fetch-site", "none")
		req.Header.Add("sec-fetch-user", "?1")
		req.Header.Add("upgrade-insecure-requests", "1")
		req.Header.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")

		resp, err := (*client.TlsClient).Do(req)
		if err != nil {
			error_channel <- "Error Making request with session client:" + err.Error()
			session_channel <- ""
			continue
		}

		resp.Body.Close()

		cookie := resp.Header["Set-Cookie"]
		session := extractSessionCookie(cookie)
		if session == "" {
			error_channel <- "No valid cookie found"
			session_channel <- ""
			continue
		}

		log.Println("Succesfully fetched Session cookie")

		session_channel <- session
	}

}

func extractSessionCookie(cookies []string) string {
	for _, cookie := range cookies {
		if strings.Contains(cookie, "_vinted_fr_session") {
			return strings.Split(strings.Split(cookie, "_vinted_fr_session=")[1], ";")[0]
		}
	}
	return ""
}

func send_webhook(u string, currency string, prod data.Item) {
	webhook := &data.Webhook{}

	// Create an embed
	embed := data.Embed{}
	var title = "Vinted Monitor (New Prod) [" + prod.Region + "]"
	embed.SetTitle(title)
	embed.SetColor(0x00ff00) // Green color
	embed.SetThumbnail(prod.Photo.URL)
	embed.SetDescription("New Product Detected ")

	embed.AddField("Title:", prod.Title, false)
	embed.AddField("Time elapsed:", fmt.Sprint(prod.TimeDiff), false)
	embed.AddField("URL: ", prod.URL, false)
	var s = "Price " + currency

	embed.AddField(s, prod.Price, false)
	var t = "Total price " + currency
	embed.AddField(t, prod.TotalItemPrice, false)
	embed.AddField("User :", prod.User.ProfileURL, false)
	embed.SetFooter(prod.Timestamp.String(), "")
	// Add the embed to the webhook
	webhook.AddEmbed(embed)

	// Send the webhook to the specified URL
	webhookURL := u
	err := webhook.Send(webhookURL)
	if err != nil {
		fmt.Println("Error sending webhook:", err)
	} else {
		fmt.Println("Webhook sent successfully")
	}
}
