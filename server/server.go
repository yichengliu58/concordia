package server

import (
	"concordia/paxos"
	"concordia/util"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var (
	pnode  *paxos.Node
	config *util.Config
	logger = util.NewLogger("<server>")
)

// for user to upload file
func fileWriter(w http.ResponseWriter, r *http.Request) {
	value := r.Header.Get(config.DigestHeader)
	if value == "" {
		logger.Debugf("request doesn't have a valid digest header")
		w.WriteHeader(400)
		w.Write([]byte("digest header not found"))
		return
	}

	dataID, err := strconv.Atoi(r.Header.Get(config.DataHeader))
	if err != nil || dataID < 0 {
		logger.Debugf("request doesn't have a valid data header")
		w.WriteHeader(400)
		w.Write([]byte("data header not valid"))
		return
	}

	clientSigOrig := r.Header.Get(config.SignatureHeader)
	var clientSig []byte
	if config.ByzantineFaultTolerance {
		if clientSigOrig == "" {
			logger.Debugf("request doesn't have a valid signature header")
			w.WriteHeader(400)
			w.Write([]byte("signature header not found"))
			return
		}
		if clientSig, err = base64.StdEncoding.DecodeString(clientSigOrig); err != nil {
			logger.Debugf("request doesn't have a valid signature header")
			w.WriteHeader(400)
			w.Write([]byte("signature header not valid"))
			return
		}
		// check client signature
		if util.Verify(value, string(clientSig), config.ClientKey) != nil {
			// failed to verify client signature
			logger.Debugf("request contains an invalid client signature")
			w.WriteHeader(400)
			w.Write([]byte("invalid signature"))
			return
		}
	}

	filename := config.FileDir + "/" + r.Header.Get(config.DigestHeader)

	// create new file, override if exists
	file, err := os.Create(filename)
	if err != nil {
		logger.Errorf("failed to create file \"%s\": %s", filename, err.Error())
		w.WriteHeader(500)
		w.Write([]byte("failed to create file"))
		return
	}

	defer file.Close()

	// write file to disk and check digest
	buf := make([]byte, config.FileBufferSize)
	_, err = io.CopyBuffer(file, r.Body, buf)
	if err != nil {
		logger.Errorf("failed to write file \"%s\": %s", filename, err.Error())
		w.WriteHeader(500)
		w.Write([]byte("failed to write file"))
		os.Remove(filename)
		return
	}

	// check digest
	file.Seek(0, 0)
	hash := md5.New()
	io.CopyBuffer(hash, file, buf)
	digest := fmt.Sprintf("%x", hash.Sum(nil))

	if digest != value {
		logger.Debugf("file uploaded digest %s does not match header %s", digest, value)
		os.Remove(filename)
		w.WriteHeader(400)
		w.Write([]byte("digest does not match"))
		return
	}

	// begin proposal
	ok, err := pnode.Propose(uint32(dataID), digest, string(clientSig))
	if !ok {
		logger.Warnf("failed to propose data %d value %s, %s", dataID, digest, err.Error())
		os.Remove(filename)
		w.WriteHeader(500)
		w.Write([]byte("failed to propose, " + err.Error()))
		return
	}
	w.WriteHeader(200)
}

func fileReader(w http.ResponseWriter, r *http.Request) {
	// get last file name
	_, name := filepath.Split(r.RequestURI)
	// search for that file
	file, err := os.Open(config.FileDir + "/" + name)
	if err != nil {
		logger.Errorf("failed to open file %s: %s", name, err.Error())
		w.WriteHeader(404)
		return
	}
	defer file.Close()

	// set content-length to force not to use chunk
	info, _ := file.Stat()
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	buf := make([]byte, config.FileBufferSize)
	s, err := io.CopyBuffer(w, file, buf)
	if err != nil {
		logger.Errorf("failed to read file %s: %s, size already read: %d",
			name, err.Error(), s)
	}
}

func fetchFile(data, log uint32, name string) {
	// try until get a file
	for _, p := range config.Peers {
		resp, err := http.Get("http://" + p.Addr.IP.String() + ":" +
			strconv.Itoa(int(config.ServicePort)) + "/files/" + name)
		if err != nil {
			logger.Debugf("failed to connect to peer %s, error: %s, retrying next one",
				p.Addr.IP.String(), err.Error())
			continue
		}

		if resp.StatusCode != 200 {
			logger.Debugf("peer %s returning not valid: status %d, retrying next one",
				p.Addr.IP.String(), resp.StatusCode, resp.ContentLength)
			continue
		}

		logger.Debugf("got file %s from peer %s", name, p.Addr.IP.String())
		defer resp.Body.Close()

		// create file
		file, err := os.Create(config.FileDir + "/" + name)
		if err != nil {
			logger.Errorf("failed to create file %s, %s", name, err.Error())
			pnode.FailCommand(data, log)
			return
		}

		buf := make([]byte, config.FileBufferSize)
		_, e := io.CopyBuffer(file, resp.Body, buf)
		if e != nil {
			logger.Errorf("failed to write file %s, %s", name, e.Error())
			pnode.FailCommand(data, log)
		} else {
			// check digest
			file.Seek(0, 0)
			digest := md5.New()
			io.CopyBuffer(digest, file, buf)
			digests := fmt.Sprintf("%x", digest.Sum(nil))
			if digests != name {
				logger.Errorf("file %s received from %s has wrong digest: %s != %s, deleting",
					name, digests, name, p.Addr.IP.String())
				os.Remove(config.FileDir + "/" + name)
				pnode.FailCommand(data, log)
			} else {
				logger.Debugf("succeeded writing file %s", name)
				pnode.ExecuteCommand(data, log)
			}
		}
		return
	}
}

// routine to update files
func update(delay time.Duration) {
	for {
		<-time.After(delay)
		for id := range pnode.DataIDInfo() {
			// check commands in this data set
			for log, name := pnode.NextCommand(id); name != ""; log, name = pnode.NextCommand(id) {
				go fetchFile(id, log, name)
			}
		}
	}
}

func Start(c *util.Config) error {
	config = c
	logger.SetOutput(config.LogOutput)
	logger.SetLevel(config.LogLevel)

	pnode = paxos.NewNode(c)
	// set up paxos
	err := pnode.Start()
	if err != nil {
		return err
	}

	go update(c.CheckingDelay)

	// set up service http server
	http.HandleFunc("/deploy", fileWriter)
	// set up protocol http server
	http.HandleFunc("/files/", fileReader)
	err = http.ListenAndServe(":"+strconv.Itoa(int(config.ServicePort)), nil)

	return err
}
