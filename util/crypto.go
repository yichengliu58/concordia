// manage private/public keys and encrypt/decrypt operations

package util

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"os"
)

// generate public and private key pairs, given a dir to put them
// keys are all 1024 bits
func GenerateKeyPair(dir string, bits int) error {
	// generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err
	}
	// encode key to file
	encoded := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: encoded,
	}
	file, err := os.Create(dir + "/privatekey.pem")
	if err != nil {
		return err
	}
	err = pem.Encode(file, block)
	if err != nil {
		// file not changed
		return err
	}
	// extract public key
	publicKey := &privateKey.PublicKey
	encoded, err = x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return err
	}
	// encode to file
	block = &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: encoded,
	}
	file, err = os.Create(dir + "/publickey.pem")
	if err != nil {
		return err
	}
	err = pem.Encode(file, block)
	if err != nil {
		return err
	}
	return nil
}

// read file and decode keys
func ParsePublicKey(file string) (*rsa.PublicKey, error) {
	// read file to get []bytes
	public, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	// decode and parse public key
	block, _ := pem.Decode(public)
	if block == nil {
		return nil, errors.New("failed to decode public key: " + file)
	}
	publickey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return publickey.(*rsa.PublicKey), nil
}

func ParsePrivateKey(file string) (*rsa.PrivateKey, error) {
	// read file to get []bytes
	private, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	// decode and parse private key
	block, _ := pem.Decode(private)
	if block == nil {
		return nil, errors.New("failed to decode public key: " + file)
	}
	privatekey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return privatekey, nil
}

// using RSA to sign some data
func Sign(data string, privateKey *rsa.PrivateKey) (string, error) {
	h := sha256.New()
	h.Write([]byte(data))
	hashed := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed)
	if err != nil {
		return "", err
	}
	return string(sig), nil
}

// verify a signed message
func Verify(data string, signature string, publicKey *rsa.PublicKey) error {
	hashed := sha256.Sum256([]byte(data))

	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], []byte(signature))
}
