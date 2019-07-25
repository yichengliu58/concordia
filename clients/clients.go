package main

import (
	"bytes"
	"concordia/util"
	"crypto/md5"
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
	stat   map[int]time.Duration
	lock   sync.Mutex
)

func request(i int, cont *bytes.Buffer, dig string, sig string, wg *sync.WaitGroup) {
	defer wg.Done()
	req, _ := http.NewRequest("POST", "http://127.0.0.1:8000/deploy", cont)
	req.Header.Add("FileDigest", dig)
	req.Header.Add("DataID", strconv.Itoa(13))
	if sig != "" {
		req.Header.Add("Signature", sig)
	}

	before := time.Now()
	resp, _ := client.Do(req)
	after := time.Now()

	if resp.StatusCode == 200 {
		lock.Lock()
		stat[i] = after.Sub(before)
		lock.Unlock()
	}
}

func main() {
	b := flag.Bool("b", false, "if byzantine fault")
	n := flag.Int("n", 5, "requests number for each concurrency")
	c := flag.Int("c", 10, "concurrent requests")
	s := flag.Int("s", 1024, "size of file content in KB")
	flag.Parse()

	content := make([]byte, *s*1024)
	// rand
	rand.Seed(time.Now().Unix())
	magic := rand.Intn(*s*1024 - 1)
	content[magic] = 90

	contbuf := bytes.NewBuffer(content)

	digest := md5.Sum(content)
	digests := fmt.Sprintf("%x", digest)

	sign := ""
	if *b {
		ck, _ := util.ParsePrivateKey("keys/client/privatekey.pem")
		sign, _ = util.Sign(string(content), ck)
	}

	for j := 0; j < *n; j++ {
		var wg sync.WaitGroup
		// start requests
		for i := 0; i < *c; i++ {
			wg.Add(1)
			go request(i, contbuf, digests, sign, &wg)
		}

		wg.Wait()

		var total time.Duration
		for _, v := range stat {
			total += v
		}
		fmt.Printf("%d ", total.Nanoseconds()/int64(len(stat)))
	}
}
