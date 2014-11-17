package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

type server struct {
	IP      string
	Port    string
	DNSName string
}

type sampleConf struct {
	Server server
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s", err.Error())
		os.Exit(1)
	}
}

func encryptWrite(conn *net.TCPConn, cipherBlock cipher.Block, plainText []byte) error {

	ciphertext := make([]byte, aes.BlockSize+len(plainText))
	initializationVector := ciphertext[:aes.BlockSize]
	_, err := io.ReadFull(rand.Reader, initializationVector)

	if err != nil {
		return err
	}

	stream := cipher.NewCTR(cipherBlock, initializationVector)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plainText)

	_, err = conn.Write(ciphertext)

	return err
}

func decryptRead(conn *net.TCPConn, cipherBlock cipher.Block, bufSize int) ([]byte, error) {
	if bufSize == 0 {
		bufSize = 8192
	}

	cipherMessage := make([]byte, bufSize)
	cipherLen, err := conn.Read(cipherMessage)
	if err != nil {
		return []byte(""), err
	}

	message := make([]byte, cipherLen-aes.BlockSize)
	stream := cipher.NewCTR(cipherBlock, cipherMessage[:aes.BlockSize])
	stream.XORKeyStream(message, cipherMessage[aes.BlockSize:cipherLen])

	return message, nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s message", os.Args[0])
		os.Exit(1)
	}
	message := os.Args[1]

	jsonFile, err := os.Open("sample.json")
	checkError(err)
	decoder := json.NewDecoder(jsonFile)
	var config sampleConf
	err = decoder.Decode(&config)
	checkError(err)

	serverIP := config.Server.IP
	serverPort := string(config.Server.Port)

	checkError(err)

	tcpAddr, err := net.ResolveTCPAddr("tcp", serverIP+":"+serverPort)
	checkError(err)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	checkError(err)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	rootBuf := make([]byte, 8192)
	rootlen, err := conn.Read(rootBuf)
	checkError(err)
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(rootBuf[:rootlen])
	if !ok {
		panic("failed to parse root certificate")
	}

	conn.Write([]byte("ok"))

	crtBuf := make([]byte, 8192)
	crtlen, err := conn.Read(crtBuf)
	checkError(err)
	block, _ := pem.Decode(crtBuf[:crtlen])
	if block == nil {
		panic("failed to parse certificate PEM")
	}
	crt, _ := x509.ParseCertificate(block.Bytes)

	options := x509.VerifyOptions{
		DNSName: config.Server.DNSName,
		Roots:   roots,
	}

	if _, err := crt.Verify(options); err != nil {
		panic("failed to verify certificate: " + err.Error())
	}
	publicKey, ok := crt.PublicKey.(*rsa.PublicKey)
	if !ok {
		panic("failed to parse public key")
	}

	sessionKey := make([]byte, 32)
	_, err = io.ReadFull(rand.Reader, sessionKey)
	checkError(err)

	cryptoKey, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey, sessionKey)
	checkError(err)

	conn.Write(cryptoKey)

	responseBuf := make([]byte, 256)
	responseLen, err := conn.Read(responseBuf)
	checkError(err)
	if string(responseBuf[:responseLen]) != "ok" {
		panic("failed to send crypto key")
	}

	cipherBlock, err := aes.NewCipher(sessionKey)
	checkError(err)

	err = encryptWrite(conn, cipherBlock, []byte(message))
	checkError(err)

	responseMessage, err := decryptRead(conn, cipherBlock, 8192)
	checkError(err)

	fmt.Println("server: " + string(responseMessage))
}
