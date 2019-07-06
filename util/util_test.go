package util

import (
	"fmt"
	"strconv"
	"testing"
)

// config tests
func TestParseConfig(t *testing.T) {
	conf, err := ParseConfig("default_conf.json")
	if err != nil {
		t.Errorf("conf parse failed: %v", err)
	}
	fmt.Println(conf)
}

// crypto tests
func TestGenerateKeyPair(t *testing.T) {
	// generate 5 key pairs
	for i := 1; i <= 5; i++ {
		err := GenerateKeyPair("../keys/"+strconv.Itoa(i), 512)
		if err != nil {
			t.Errorf("failed to generate key pairs: %v", err)
		}
	}
	err := GenerateKeyPair("../keys/client", 512)
	if err != nil {
		t.Errorf("failed to generate key pairs: %v", err)
	}
}

func TestParsePublicKey(t *testing.T) {
	for i := 1; i < 5; i++ {
		key, err := ParsePublicKey("../keys/" + strconv.Itoa(i) + "/publickey.pem")
		if err != nil {
			t.Errorf("failed to parse key: %s, err %v",
				"../keys/"+strconv.Itoa(i), err)
		} else {
			t.Logf("key %d parsed", key.E)
		}
	}
}

func TestParsePrivateKey(t *testing.T) {
	for i := 1; i < 5; i++ {
		key, err := ParsePrivateKey("../keys/" + strconv.Itoa(i) + "/privatekey.pem")
		if err != nil {
			t.Errorf("failed to parse key: %s, err %v",
				"../keys/"+strconv.Itoa(i)+"/privatekey.pem", err)
		} else {
			t.Logf("key %d parsed", key.E)
		}
	}
}

func TestSign(t *testing.T) {
	key, err := ParsePrivateKey("../keys/" + strconv.Itoa(1) + "/privatekey.pem")
	if err != nil {
		t.Errorf("failed to parse key %s, err %v",
			"../keys/"+strconv.Itoa(1)+"/privatekey.pem", err)
	}
	secret, err := Sign("he is not living long", key)
	if err != nil {
		t.Errorf("failed to sign: %v", err)
	}
	t.Logf("signature: %s", secret)
}

func TestVerify(t *testing.T) {
	key, err := ParsePrivateKey("../keys/" + strconv.Itoa(1) + "/privatekey.pem")
	pkey, err := ParsePublicKey("../keys/" + strconv.Itoa(1) + "/publickey.pem")
	if err != nil {
		t.Errorf("failed to parse key %s, err %v",
			"../keys/"+strconv.Itoa(1)+"/privatekey.pem", err)
	}
	sig, err := Sign("he is not living long", key)
	if err != nil {
		t.Errorf("failed to sign: %v", err)
	}
	err = Verify("he is not living long", sig, pkey)
	if err != nil {
		t.Errorf("failed to verify: %v", err)
	}
}
