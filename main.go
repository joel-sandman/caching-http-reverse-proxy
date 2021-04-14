package main

import (
	"bytes"
    "fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
    "os"
    "strconv"
    "time"

    "github.com/hashicorp/terraform/helper/hashcode"
    "github.com/patrickmn/go-cache"
)

const (
	csvFileName             = "data.csv"
	memoryUsageCsvFileName  = "memory-data.csv"
)

func main() {
    expiration, err := strconv.Atoi(os.Getenv("TTL"))
	if err != nil {
		log.Fatalf("TTL cannot be parsed as integer")
	}

    port, err := strconv.Atoi(os.Getenv("PROXY_LISTEN_PORT"))
	if err != nil {
		log.Fatalf("PROXY_LISTEN_PORT cannot be parsed as integer")
	}

	path, err := url.Parse(fmt.Sprintf("http://%s", os.Getenv("FRONTEND_ADDR")))
	if err != nil {
		log.Fatalf("Could not parse downstream url: %s", path)
	}

    proxyCache := *cache.New(10*time.Second, 60*time.Second)

    memoryUsageCsvFile, err := os.Create(memoryUsageCsvFileName)
	if err != nil {
		log.Fatalf("Could not open CSV file (%s) for writing", memoryUsageCsvFile)
	}
	defer memoryUsageCsvFile.Close()
    go memoryUsageStatus(proxyCache, log.New(memoryUsageCsvFile, "", 0))

    csvFile, err := os.Create(csvFileName)
	if err != nil {
		log.Fatalf("Could not open CSV file (%s) for writing", csvFileName)
	}
	defer csvFile.Close()
    csvLog := log.New(csvFile, "", 0)
    csvLog.Printf("timestamp,source,info,size,url(hash)\n")

    handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        reqBody, err := ioutil.ReadAll(req.Body)
        if err != nil {
            log.Printf("Error reading req body")
        }
        err = req.Body.Close()
        if err != nil {
            log.Printf("Error closing req body")
        }
        reqUrl := req.URL.String()
        reqSize := len(reqBody)

        hash := hashcode.Strings([]string{reqUrl, string(reqBody)})
        proxy := httputil.NewSingleHostReverseProxy(path)
        proxy.ModifyResponse = func(res *http.Response) error {
            resBody, err := ioutil.ReadAll(res.Body)
            if err != nil {
                return err
            }
            err = res.Body.Close()
            if err != nil {
                return err
            }
            if cachedValue, found := proxyCache.Get(hash); found {
                // log.Printf("Found in cache!")
                cachedValueString := fmt.Sprintf("%v", cachedValue)

                resSize := len(resBody)
                totSize := reqSize + resSize

                match := "stale"
                if string(resBody) == cachedValueString {
                    match = "fresh"
                    log.Printf("Fresh data in cache")
                } else {
                    log.Printf("Stale data in cache")
                }
                csvLog.Printf("%d,cache,%s,%d,%s(%s)\n", time.Now().UnixNano(), match, totSize, reqUrl, hash)
            } else {
                // log.Printf("NOT found in cache!")
                resSize := len(resBody)
                totSize := reqSize + resSize

                proxyCache.Set(hash, string(resBody), time.Duration(expiration)*time.Millisecond)
                csvLog.Printf("%d,downstream,,%d,%s(%s)\n", time.Now().UnixNano(), totSize, reqUrl, hash)
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

    log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), handler))
}

func memoryUsageStatus(proxyCache cache.Cache, csvLog *log.Logger) {
	csvLog.Printf("timestamp,items,bytes")
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

		csvLog.Printf("%d,%d,%d", time.Now().UnixNano(), items, size)
	}
}
