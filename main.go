package main

import (
	"bytes"
    "fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
    "strconv"
    "time"

    "github.com/hashicorp/terraform/helper/hashcode"
    "github.com/patrickmn/go-cache"
)

func main() {
	port := ":1234"
	u, err := url.Parse("http://localhost:8080")
	if err != nil {
		log.Fatalf("Could not parse downstream url: %s", u)
	}

    proxyCache := *cache.New(10*time.Second, 60*time.Second)
    expiration := 60000

    go memoryUsageStatus(proxyCache)

    handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        reqBody, err := ioutil.ReadAll(req.Body)
        if err != nil {
            log.Printf("Error reading req body")
        }
        err = req.Body.Close()
        if err != nil {
            log.Printf("Error closing req body")
        }
        reqURL := req.URL.String()
        hash := hashcode.Strings([]string{reqURL, string(reqBody)})
        proxy := httputil.NewSingleHostReverseProxy(u)
        proxy.ModifyResponse = func(res *http.Response) error {
            resBody, err := ioutil.ReadAll(res.Body)
            if err != nil {
                return  err
            }
            err = res.Body.Close()
            if err != nil {
                return err
            }
            if cachedValue, found := proxyCache.Get(hash); found {
                log.Printf("Found in cache!")
                cachedValueString := fmt.Sprintf("%v", cachedValue)
                // match := "stale"
                if string(resBody) == cachedValueString {
                    // match = "fresh"
                    log.Printf("Fresh data in cache")
                } else {
                    log.Printf("Stale data in cache")
                }
            } else {
                log.Printf("NOT found in cache!")
                proxyCache.Set(hash, string(resBody), time.Duration(expiration)*time.Millisecond)
            }
        
            newResbody := ioutil.NopCloser(bytes.NewReader(resBody))
            res.Body = newResbody
            res.ContentLength = int64(len(resBody))
            res.Header.Set("Content-Length", strconv.Itoa(len(resBody)))
            return nil
        }
        newReqbody := ioutil.NopCloser(bytes.NewReader(reqBody))
        req.Body = newReqbody
        req.ContentLength = int64(len(reqBody))
        req.Header.Set("Content-Length", strconv.Itoa(len(reqBody)))
        proxy.ServeHTTP(w, req)
    })

    log.Fatal(http.ListenAndServe(port, handler))
}

func memoryUsageStatus(proxyCache cache.Cache) {
	// csvLog.Printf("timestamp,items,bytes")
	for {
		time.Sleep(15 * time.Second)
		proxyCache.DeleteExpired()
		items := proxyCache.ItemCount()
		log.Printf("Items in cache: %d", items)

		var buf bytes.Buffer
		err := proxyCache.Save(&buf)
		if err != nil {
			log.Printf("Failed save cache to buffer")
		}
		size := buf.Len()
		log.Printf("Size of cache (bytes): %d", size)

		// csvLog.Printf("%d,%d,%d", time.Now().UnixNano(), items, size)
	}
}
