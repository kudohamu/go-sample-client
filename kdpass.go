package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

type server struct {
	IP   string
	Port string
}

type kdpassConf struct {
	Server server
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s message", os.Args[0])
		os.Exit(1)
	}
	message := os.Args[1]

	jsonFile, err := os.Open("kdpass.json")
	checkError(err)
	decoder := json.NewDecoder(jsonFile)
	var config kdpassConf
	err = decoder.Decode(&config)
	checkError(err)

	serverIP := config.Server.IP
	serverPort := string(config.Server.Port)

	tcpAddr, err := net.ResolveTCPAddr("tcp", serverIP+":"+serverPort)
	checkError(err)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	checkError(err)
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.Write([]byte(message))

	readBuf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	readlen, err := conn.Read(readBuf)
	checkError(err)

	fmt.Println("server: " + string(readBuf[:readlen]))
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s", err.Error())
		os.Exit(1)
	}
}
