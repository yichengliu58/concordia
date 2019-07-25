package main

import (
	"bytes"
	"concordia/util"
	"crypto/md5"
	"encoding/base64"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var (
	client http.Client
	stat   = make(map[int]time.Duration)
	lock   sync.Mutex
)

func request(i int, s int, b bool, wg *sync.WaitGroup) {
	defer wg.Done()

	content := make([]byte, s*1024)
	// rand
	rand.Seed(time.Now().Unix())
	magic := rand.Intn(s*1024 - 1)
	content[magic] = 90

	digest := md5.Sum(content)
	digests := fmt.Sprintf("%x", digest)

	sign := ""
	if b {
		ck, _ := util.ParsePrivateKey("keys/client/privatekey.pem")
		sign, _ = util.Sign(string(content), ck)
	}

	cont := bytes.NewBuffer(content)
	req, _ := http.NewRequest("POST", "http://127.0.0.1:8000/deploy", cont)
	req.Header.Add("FileDigest", digests)
	req.Header.Add("DataID", strconv.Itoa(13))
	if sign != "" {
		esig := base64.StdEncoding.EncodeToString([]byte(sign))
		req.Header.Add("Signature", esig)
	}

	before := time.Now()
	resp, err := client.Do(req)
	after := time.Now()

	if err != nil {
		fmt.Println(err)
		return
	} else if resp.StatusCode == 200 {
		lock.Lock()
		stat[i] = after.Sub(before)
		lock.Unlock()
	} else {
		fmt.Println("unsuccessful resp:", resp.StatusCode)
		return
	}
}

func main() {
	b := flag.Bool("b", false, "if byzantine fault")
	n := flag.Int("n", 5, "requests number for each concurrency")
	c := flag.Int("c", 10, "concurrent requests")
	s := flag.Int("s", 1024, "size of file content in KB")
	flag.Parse()

	for j := 0; j < *n; j++ {
		var wg sync.WaitGroup
		// start requests
		for i := 0; i < *c; i++ {
			wg.Add(1)
			go request(i, *s, *b, &wg)
		}

		wg.Wait()

		var total time.Duration
		for _, v := range stat {
			total += v
		}
		fmt.Printf("%d ", total.Nanoseconds()/int64(len(stat))/1000000)
	}
	fmt.Println()
}
