package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/redis/go-redis/v9"
)

// const endpoint4 = "http://ip1.dynupdate.no-ip.com"
const endpoint6 = "http://ip1.dynupdate6.no-ip.com"
const redisHost = "redis.appsite.top:6379"
const redisIpv6Key = "cfsync:ipv6"

const baseUrl = "https://api.cloudflare.com/client/v4/zones/"

var cfzone string
var cftoken string
var redispass string
var ctx = context.Background()
var rdb *redis.Client

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
		SetBaseURL(baseUrl + cfzone).
		SetAuthToken(cftoken)
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
		SetBaseURL(baseUrl + cfzone + "/dns_records").
		SetAuthToken(cftoken)

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
	cfzone = os.Getenv("CFZONE")
	cftoken = os.Getenv("CFTOKEN")
	redispass = os.Getenv("REDISPASS")
	if len(cfzone) < 1 || len(cftoken) < 1 || len(redispass) < 1 {
		log.Printf("cfzone: %s cftoken: %s redispass %s", cfzone, cftoken, redispass)
		log.Fatal("zone nor cftoken nor redispass is empty")
	}
	rdb = redis.NewClient(&redis.Options{
		Addr:      redisHost,
		Username:  "cfsync",
		Password:  redispass, // no password set
		DB:        0,         // use default DB
		TLSConfig: &tls.Config{},
	})
}

func judgeIfShouldUpdate(ip string) {
	val, err := rdb.Get(ctx, redisIpv6Key).Result()
	if err == redis.Nil {
		rdb.Set(ctx, redisIpv6Key, ip, 0)
	} else {
		if err != nil {
			log.Fatal("redis error:", err)
		}
		if val == ip {
			log.Printf("same ip %s, skip update cloudflare", ip)
			os.Exit(0)
		} else {
			rdb.Set(ctx, redisIpv6Key, ip, 0)
		}
	}
}

func main() {
	initEnv()
	ip, err := getIp()
	if err != nil {
		log.Fatal("failed to get ipv6", err)
	}
	log.Println("ipv6 address:", ip)
	if len(ip) < 1 {
		log.Fatal("not a good ipv6")
	}
	judgeIfShouldUpdate(ip)
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
