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
	UK_BASE_URL = "https://www.vinted.co.uk"
	FR_BASE_URL = "https://www.vinted.fr"
	DE_BASE_URL = "https://www.vinted.de"
	PL_BASE_URL = "https://www.vinted.pl"
)

var session_channel chan string
var error_channel chan string
var product_channel chan data.Message

var proxies []string

type Monitor struct {
	Client       Client
	FOUND_SKU    []int
	UrlToMonitor string
	Currency     string
	Webhook      string
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
func NewMonitor(link string, clientUrl string, currency string, webhook string) (*Monitor, error) {
	client, err := NewClient(clientUrl, clientUrl[len(clientUrl)-2:])

	if err != nil {
		return nil, err
	}
	monitor := Monitor{
		Client:       *client,
		FOUND_SKU:    []int{},
		UrlToMonitor: link,
		Currency:     currency,
		Webhook:      webhook,
	}
	return &monitor, nil
}
func main() {
	var monitors []*Monitor

	error_channel = make(chan string)
	session_channel = make(chan string)
	product_channel = make(chan data.Message)

	links := parse_url_file()
	parse_proxy_file()
	webhooks := parse_webhook_file()

	if len(links) != len(webhooks) {
		log.Fatal("Differences between links and webhooks files")
	}

	// MONITOR CREATION FOR EACH LINK AND WEBHOOK
	for i, url := range links {

		client_base_url := strings.Split(url, "/api")[0]
		if client_base_url == UK_BASE_URL {
			monitor, err := NewMonitor(url, UK_BASE_URL, UK_CURRENCY, webhooks[i])
			if err != nil {
				log.Printf("Error creating monitor struct at: %s, at index: %d", url, i)
			}
			monitors = append(monitors, monitor)
			continue
		}
		if client_base_url == FR_BASE_URL {
			monitor, err := NewMonitor(url, FR_BASE_URL, FR_CURRENCY, webhooks[i])
			if err != nil {
				log.Printf("Error creating monitor struct at: %s, at index: %d", url, i)
			}
			monitors = append(monitors, monitor)
			continue
		}
		if client_base_url == DE_BASE_URL {
			monitor, err := NewMonitor(url, DE_BASE_URL, DE_CURRENCY, webhooks[i])
			if err != nil {
				log.Printf("Error creating monitor struct at: %s, at index: %d", url, i)
			}
			monitors = append(monitors, monitor)
			continue
		}
		if client_base_url == PL_BASE_URL {
			monitor, err := NewMonitor(url, PL_BASE_URL, PL_CURRENCY, webhooks[i])
			if err != nil {
				log.Printf("Error creating monitor struct at: %s, at index: %d", url, i)
			}
			monitors = append(monitors, monitor)
			continue
		}

	}

	session_client, err := NewClient("https://www.vinted.com", "All")
	if err != nil {
		log.Fatal("Error creating client to fetch session")
	}
	session_client2, err := NewClient("https://www.vinted.com", "All")
	if err != nil {
		log.Fatal("Error creating client to fetch session")
	}
	go get_session(session_client)
	go get_session(session_client2)
	for _, monitor := range monitors {
		go monitor.new_product_monitor(&monitor.Client)
	}

	go func() {
		for {
			select {
			case errMsg := <-error_channel:
				fmt.Println("Error:", errMsg)

			case MessageProd := <-product_channel:
				send_webhook(MessageProd.Webhook, MessageProd.Currency, MessageProd.Item)
			}

		}
	}()
	// Keep the main function running
	select {}

}
func parse_webhook_file() []string {
	var webhook_links []string
	file, err := os.Open("webhooks.txt")
	if err != nil {
		error_channel <- err.Error()
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		link := scanner.Text()
		link = strings.TrimSpace(link)
		webhook_links = append(webhook_links, link)

	}
	return webhook_links
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
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		proxy := formatProxy(line)
		proxies = append(proxies, proxy)
	}
}

// add other regions
func parse_url_file() []string {
	var links []string
	file, err := os.Open("links.txt")
	if err != nil {
		error_channel <- err.Error()
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var url string
		line := scanner.Text()
		new := strings.Split(line, "?")
		var query string
		if !strings.Contains(new[1], "catalog_ids") {
			query = strings.Replace(new[1], "catalog", "catalog_ids", 1)

		} else {

			query = new[1]
		}
		if strings.Contains(line, "https://www.vinted.co.uk") {
			url = UK_BASE_URL + "/api/v2/catalog/items?" + query
		}
		if strings.Contains(line, "https://www.vinted.fr") {
			url = FR_BASE_URL + "/api/v2/catalog/items?" + query
		}
		if strings.Contains(line, "https://www.vinted.de") {
			url = DE_BASE_URL + "/api/v2/catalog/items?" + query
		}
		if strings.Contains(line, "https://www.vinted.pl") {
			url = PL_BASE_URL + "/api/v2/catalog/items?" + query
		}
		if strings.Contains(url, "catalog_ids") {
			links = append(links, url)
		} else {
			url = strings.Replace(url, "catalog", "catalog_ids", 1)
			links = append(links, url)
		}

	}
	return links

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
				continue
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
					item.StringTime = time.Now().Format(time.RFC3339)
					Message := data.Message{
						Item:     item,
						Currency: m.Currency,
						Webhook:  m.Webhook,
					}
					product_channel <- Message
					log.Println("["+m.Client.Region+"]"+"Found New Item with SKU: ", item.ID)
					//DropTime := time.Now()
					//item.Timestamp = DropTime
					// NO NEED TO FIRE THIS IF NOT FOR TESTING PURPOSES
					//go get_product_timestamp(*client.TlsClient, client.url+"/api/v2/items/", item.ID, session, item, m)

				}
			}

		} else {
			error_channel <- fmt.Sprintf("["+m.Client.Region+"]"+"Status Code [%d].  "+m.UrlToMonitor, resp.StatusCode)
			time.Sleep(2 * time.Second)

			continue
		}

	}

}
func get_product_timestamp(client tls_client.HttpClient, api_Url string, Sku int64, session string, item data.Item, monitor *Monitor) {
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

		// MESSAGE STRUCT WITH EVERYTHING INSIDE

		Message := data.Message{
			Item:     item,
			Currency: monitor.Currency,
			Webhook:  monitor.Webhook,
		}
		product_channel <- Message

		log.Println("item sent closing goroutine")
		return

	}

}
func get_session(client *Client) {
	for {
		rotate_proxy(*client.TlsClient)
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
			continue
		}

		resp.Body.Close()

		cookie := resp.Header["Set-Cookie"]
		session := extractSessionCookie(cookie)
		if session == "" {
			error_channel <- "No valid cookie found"
			continue
		}

		log.Println("Succesfully fetched Session cookie : %s", session[1:10])

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
	//embed.AddField("Time elapsed:", fmt.Sprint(prod.TimeDiff), false)
	embed.AddField("URL: ", prod.URL, false)
	var s = "Price " + currency

	embed.AddField(s, prod.Price, false)
	var t = "Total price " + currency
	embed.AddField(t, prod.TotalItemPrice, false)
	embed.AddField("User :", prod.User.ProfileURL, false)
	embed.SetFooter(prod.StringTime, "")
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
