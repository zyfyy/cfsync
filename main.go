package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/go-resty/resty/v2"
)

// const endpoint4 = "http://ip1.dynupdate.no-ip.com"
const endpoint6 = "http://ip1.dynupdate6.no-ip.com"

const baseUrl = "https://api.cloudflare.com/client/v4/zones/"

var zone string
var token string

func getIp() (string, error) {
	client := resty.New()
	log.Println("request", endpoint6)

	resp, err := client.R().Get(endpoint6)
	if err != nil {
		return "", err
	}

	return string(resp.Body()), nil
}

func listRecords() ([]string, error) {
	var res ListRecords
	ids := make([]string, 0)
	client := resty.New().
		SetBaseURL(baseUrl + zone).
		SetAuthToken(token)
	_, err := client.R().SetResult(&res).Get("/dns_records")
	if err != nil {
		log.Fatalf(err.Error())
		return ids, err
	}

	for _, v := range res.Result {
		if v.Type == "AAAA" {
			ids = append(ids, v.ID)
		}
	}
	return ids, nil
}

func updateRecords(id string, ip string, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println("patch", id)
	client := resty.New().
		SetBaseURL(baseUrl + zone + "/dns_records").
		SetAuthToken(token)

	patchByte, _ := json.Marshal(PatchBody{
		Content: ip,
	})

	resp, err := client.R().
		SetBody(string(patchByte)).
		Patch(id)

	if err != nil {
		log.Fatalf(err.Error())
	}

	if resp.StatusCode() != 200 {
		log.Println(string(resp.Body()))
	}
}

func initEnv() {
	zone = os.Getenv("ZONE")
	token = os.Getenv("TOKEN")
	if !(len(zone) > 0 && len(token) > 0) {
		log.Fatal("zone nor token is empty")
	}
}

func main() {
	initEnv()
	ip, err := getIp()
	if err != nil {
		log.Fatal("failed to get ipv6", err)
	}
	log.Println(ip)
	ids, err := listRecords()
	if err != nil {
		log.Println(err)
	}
	var wg sync.WaitGroup
	for _, v := range ids {
		wg.Add(1)
		go updateRecords(v, ip, &wg)
	}
	wg.Wait()
}
