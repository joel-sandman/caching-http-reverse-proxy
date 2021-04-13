package main

// import (
// 	"log"
//     "net/http"
//     "net/url"
//     "github.com/cssivision/reverseproxy"
// )

import (
	"bytes"
	// "encoding/json"
    // "fmt"
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

var Cache cache.Cache
var expiration int

func rewriteBody(resp *http.Response) (err error) {
    b, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return  err
    }
    err = resp.Body.Close()
    if err != nil {
        return err
    }

    hash := hashcode.Strings([]string{string(b)})
    if _, found := Cache.Get(hash); found {
        log.Printf("Found in cache!")
    } else {
        log.Printf("NOT found in cache!")
        Cache.Set(hash, b, time.Duration(expiration)*time.Millisecond)
    }

    body := ioutil.NopCloser(bytes.NewReader(b))
    resp.Body = body
    resp.ContentLength = int64(len(b))
    resp.Header.Set("Content-Length", strconv.Itoa(len(b)))
    return nil
}

// func main() {
//     handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

//         log.Printf("Request: %s", r)
//         // log.Printf("ResponseWriter: %s", w)

//         // transport := http.DefaultTransport
//         // resp, err := transport.RoundTrip(r)
//         // if resp != nil && err == nil {
//         //     log.Printf("Error with roundtrip")
//         // }
//         // log.Printf("Response body: %s", resp.Body)

//         path, err := url.Parse("http://localhost:8080")
//         if err != nil {
//             panic(err)
//             return
//         }
//         proxy := reverseproxy.NewReverseProxy(path)
//         proxy.ServeHTTP(w, r)

//         // log.Printf("Request2: %s", r)
//         // log.Printf("ResponseWriter2: %s", w)
//     })

//     http.ListenAndServe(":1234", handler)
// }

func main() {
    go memoryUsageStatus()
	port := ":1234"
	u, err := url.Parse("http://localhost:8080")
	if err != nil {
		log.Fatalf("Could not parse downstream url: %s", u)
	}

    Cache = *cache.New(10*time.Second, 60*time.Second)
    expiration = 60000

	proxy := httputil.NewSingleHostReverseProxy(u)
	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		director(req)
	}
    proxy.ModifyResponse = rewriteBody

	http.HandleFunc("/", proxy.ServeHTTP)
	log.Fatal(http.ListenAndServe(port, nil))
}

func memoryUsageStatus() {
	// csvLog.Printf("timestamp,items,bytes")
	for {
		time.Sleep(15 * time.Second)
		Cache.DeleteExpired()
		items := Cache.ItemCount()
		log.Printf("Items in cache: %d", items)

		var buf bytes.Buffer
		err := Cache.Save(&buf)
		if err != nil {
			log.Printf("Failed save cache to buffer")
		}
		size := buf.Len()
		log.Printf("Size of cache (bytes): %d", size)

        // allItems := Cache.Items()
        // for key, element := range allItems {
        //     log.Println("Key:", key, "=>", "Element:", element)
        // }

		// csvLog.Printf("%d,%d,%d", time.Now().UnixNano(), items, size)
	}
}