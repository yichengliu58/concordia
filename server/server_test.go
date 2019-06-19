package server

import (
	"bytes"
	"concordia/util"
	"crypto/md5"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

var (
	defaultConfig = util.Config{
		ID:                  1,
		ProtoPort:           10000,
		ServicePort:         8000,
		Peers:               make([]net.TCPAddr, 0),
		MaxPendingProposals: 100,
		PrepareTimeout:      time.Second * 5,
		AcceptTimeout:       time.Second * 5,
		AcceptorTimeout:     time.Second * 5,
		LogOutput:           os.Stdout,
		LogLevel:            util.ERROR,
		QuorumNumber:        3,
		FileDir:             ".",
		DigestHeader:        "FileDigest",
		DataHeader:          "DataID",
		FileBufferSize:      2048,
		CheckingDelay:       time.Second * 3,
	}
)

func TestFileWriter(t *testing.T) {
	// set up service http server
	config = &defaultConfig
	http.HandleFunc("/deploy", fileWriter)

	go func() {
		err := http.ListenAndServe(":"+strconv.Itoa(int(config.ServicePort)), nil)
		if err != nil {
			t.Fatalf("failed to start server: %s", err.Error())
		}
	}()

	file := bytes.NewBuffer([]byte("this is the content of a test file"))
	digest := md5.Sum(file.Bytes())
	digests := fmt.Sprintf("%x", digest)
	t.Logf("digest: %s", digests)

	respcontent := make([]byte, 100)

	// test without digest header
	file = bytes.NewBuffer([]byte("this is the content of a test file"))
	resp, err := http.Post("http://127.0.0.1:8000/deploy", "application/octet-stream", file)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if resp.StatusCode != 400 {
		s, _ := resp.Body.Read(respcontent)
		t.Fatalf("response status %d content %s", resp.StatusCode, respcontent[:s])
	}

	// test a wrong digest value
	file = bytes.NewBuffer([]byte("this is the content of a test file"))
	req, _ := http.NewRequest("POST", "http://127.0.0.1:8000/deploy", file)
	req.Header.Add("FileDigest", "xxxx")
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if resp.StatusCode != 400 {
		s, _ := resp.Body.Read(respcontent)
		t.Fatalf("response status %d content %s", resp.StatusCode, respcontent[:s])
	}

	// test if a file is created with correct content
	file = bytes.NewBuffer([]byte("this is the content of a test file"))
	req, _ = http.NewRequest("POST", "http://127.0.0.1:8000/deploy", file)
	req.Header.Add("FileDigest", digests)
	req.Header.Add("DataID", strconv.Itoa(1))
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if resp.StatusCode != 200 {
		s, _ := resp.Body.Read(respcontent)
		t.Fatalf("response status %d content %s", resp.StatusCode, respcontent[:s])
	}

	nfile, err := os.Open("./" + fmt.Sprintf("%x", digest))
	if err == os.ErrNotExist {
		t.Fatalf("file not created")
	}
	defer nfile.Close()
}

func TestFileReader(t *testing.T) {
	// set up service http server
	config = &defaultConfig
	http.HandleFunc("/files/", fileReader)

	go func() {
		err := http.ListenAndServe(":"+strconv.Itoa(int(config.ServicePort)), nil)
		if err != nil {
			t.Fatalf("failed to start server: %s", err.Error())
		}
	}()

	file := bytes.NewBuffer([]byte("this is the content of a test file"))
	digest := md5.Sum(file.Bytes())
	digests := fmt.Sprintf("%x", digest)
	resp, err := http.Get("http://127.0.0.1:8000/files/" + digests)

	if err != nil {
		t.Fatalf("failed to get file: %s", err.Error())
	}

	if resp.StatusCode != 200 {
		t.Fatalf("recieved status %d not 200", resp.StatusCode)
	}

	content := make([]byte, 100)
	s, _ := resp.Body.Read(content)
	t.Logf("%s", content[:s])
}
