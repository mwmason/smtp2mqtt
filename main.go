package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"fmt"
	"log"
	"net/mail"
	"strings"

	"example.com/smtpd"
)

var (
	enableDebug  = flag.Bool("debug", false, "Enable debug messages")
	optlogfile   = flag.String("log","","Write logs to named file")
	welcomeMsg   = flag.String("welcome", "MQTT-forwarder ESMTP ready.", "Welcome message for SMTP session")
	mqttServer   = flag.String("mqtt", "tcp://127.0.0.1:1883", "connect to specified MQTT server")
	mqttUser     = flag.String("user", "", "MQTT username for connecting")
	mqttPassword = flag.String("password", "", "MQTT password for connecting")
	mqttJSON     = flag.Bool("json", false, "post to MQTT topic as json")
	mqttKeep     = flag.Bool("keep", false, "keep connection to MQTT")
	mqttTopic    = flag.String("topic", "", "prepend specified string to MQTT topic (e.g. 'smtp/')")
	listenStr    = flag.String("listen", "0.0.0.0:10025", "Listen on specific IP and port")
	allowStr     = flag.String("allow", "", "Allow only specific IPs to send e-mail (e.g. 192.168.1.)")
	denyStr      = flag.String("deny", "", "Deny specific IPs to send e-mail (e.g. 192.168.1.10)")
	displayVer   = flag.Bool("version", false, "Display version")
)

func smtphandler(peer smtpd.Peer, env smtpd.Envelope) error {
    var msg *mail.Message

	type MailJSON struct {
		Subject    string   `json:"subject"`
		Sender     string   `json:"sender"`
		Recipients []string `json:"recipients"`
		EmailText  string   `json:"text"`
		EmailHTML  string   `json:"html"`
	}

	if *enableDebug {
		log.Printf("env.Data: %s", env.Data)
	}

	r := bytes.NewReader(env.Data)

	msg, err := mail.ReadMessage(r)
	if err != nil {
		log.Printf("error parsing env.Data: %v", err)
		return nil
	}

	subject, emailHTML, emailText, emailIsMultiPart, errMsg := parseMsg(msg)
	if errMsg != nil {
		log.Printf("error parsing msg: %v", errMsg)
		return nil
	}

	if *enableDebug {
		log.Printf("env.Sender: %s", env.Sender)
		log.Printf("env.Recipients: %s", env.Recipients)
		log.Printf("subject: %s", subject)
		log.Printf("email text: %s", emailText)
		log.Printf("html text: %s", emailHTML)
		log.Printf("email multipart: %v", emailIsMultiPart)
	}

	if *mqttJSON {
		mailjson := MailJSON{
			Subject:    subject,
			Sender:     env.Sender,
			Recipients: env.Recipients,
			EmailText:  string(emailText),
			EmailHTML:  string(emailHTML),
		}
		if *enableDebug {
			log.Printf("JSON struct: %v", mailjson)
		}
		jsonb, errj := json.Marshal(mailjson)
		if errj != nil {
			log.Printf("error marshalling JSON: %v", errj)
		}
		if *enableDebug {
			log.Printf("JSON encoded: %s", string(jsonb))
		}
		mqttFinalTopic := *mqttTopic
		log.Printf("E-mail received from %s posting with topic %s", env.Sender, mqttFinalTopic)
		send2mqtt(mqttFinalTopic, string(jsonb))
	} else {
		mqttSubject := sanitizeTopic(subject)
		mqttFinalTopic := *mqttTopic + mqttSubject
		log.Printf("E-mail received from %s posting with topic %s", env.Sender, mqttFinalTopic)
		send2mqtt(mqttFinalTopic, string(emailText))
	}

	return nil
}

func checkhello(peer smtpd.Peer, name string) error {
	if *allowStr != "" {
		allow := *allowStr
		if !strings.HasPrefix(peer.Addr.String(), allow) {
			log.Printf("Denying Helo from %s because of allow only %s", peer.Addr.String(), *allowStr)
			return errors.New("Denied")
		}
	}
	if *denyStr != "" {
		deny := *denyStr
		if strings.HasPrefix(peer.Addr.String(), deny) {
			log.Printf("Denying Helo from %s because of deny %s", peer.Addr.String(), *denyStr)
			return errors.New("Denied")
		}
	}
	log.Printf("Accepting Helo from: %s", peer.Addr.String())
	return nil
}

func main() {
	var server *smtpd.Server

	flag.Parse()

	if *optlogfile != "" {
		logFile, err := os.OpenFile(*optlogfile, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			log.Panic(err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	if *displayVer {
		fmt.Printf("version %s - %s\n", Version, CommitID)
		os.Exit(1);
	}

	// No-op server. Accepts and discards
	server = &smtpd.Server{
		WelcomeMessage: *welcomeMsg,
		Handler:        smtphandler,
		HeloChecker:    checkhello,
	}
	err := initmqtt()
	if err != nil {
		log.Printf("Error connecting to MQTT server %s: %v", *mqttServer, err)
		return
	}
	log.Printf("Listening on: %s", *listenStr)
	errl := server.ListenAndServe(*listenStr)
	if errl != nil {
		log.Printf("Error listening on '%s': %v", *listenStr, errl)
	}
	closemqtt()
}
