package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/mail"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"strings"
)

var headerSplitter = []byte("\r\n\r\n")

func parseMsg(msg *mail.Message) (subject string, html string, text string, isMultipart bool, err error) {

	// Display only the main headers of the message. The "From","To" and "Subject" headers
	// have to be decoded if they were encoded using RFC 2047 to allow non ASCII characters.
	// We use a mime.WordDecode for that.
	dec := new(mime.WordDecoder)
	from, _ := dec.DecodeHeader(msg.Header.Get("From"))
	to, _ := dec.DecodeHeader(msg.Header.Get("To"))
	subject, _ = dec.DecodeHeader(msg.Header.Get("Subject"))
	if *enableDebug {
		log.Println("From:", from)
		log.Println("To:", to)
		log.Println("Date:", msg.Header.Get("Date"))
		log.Println("Subject:", subject)
		log.Println("Content-Type:", msg.Header.Get("Content-Type"))
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		log.Fatal(err)
	}

	if !strings.HasPrefix(mediaType, "multipart/") {
		log.Println("Not a multipart MIME message")
		text = ""
		isMultipart = false
		return
	}

	// Recursivey parsed the MIME parts of the Body, starting with the first
	// level where the MIME parts are separated with params["boundary"].
	isMultipart = true
	html, text, err = ParsePart(msg.Body, params["boundary"])
	return

}


func ParsePart(mime_data io.Reader, boundary string) (html string, text string, errPart error) {
	// Instantiate a new io.Reader dedicated to MIME multipart parsing
	// using multipart.NewReader()
	body := []byte("\n")
	html = ""
	text = ""
	reader := multipart.NewReader(mime_data, boundary)
	if reader == nil {
		return
	}
	// Go through each of the MIME part of the message Body with NextPart(),
	for {
		new_part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error going through the MIME parts -", err)
			break
		}
		mediaType, params, err := mime.ParseMediaType(new_part.Header.Get("Content-Type"))
		if err == nil && strings.HasPrefix(mediaType, "multipart/") {
			// This is a new multipart to be handled recursively
			return ParsePart(new_part, params["boundary"])
		} else {
			part, err := ioutil.ReadAll(new_part)
			if err != nil {
				break
			}
			content_transfer_encoding := strings.ToUpper(new_part.Header.Get("Content-Transfer-Encoding"))
			log.Printf("Got mediatype %s and content-encoding %s for part %s", mediaType, content_transfer_encoding, string(part))
			switch {
					case strings.Compare(content_transfer_encoding, "BASE64") == 0:
						decoded_content, err := base64.StdEncoding.DecodeString(string(part))
						if err != nil {
							log.Println("Error decoding base64 -", err)
						} else {
							body = decoded_content
						}
					case strings.Compare(content_transfer_encoding, "QUOTED-PRINTABLE") == 0:
						decoded_content, err := ioutil.ReadAll(quotedprintable.NewReader(bytes.NewReader(part)))
						if err != nil {
							log.Println("Error decoding quoted-printable -", err)
						} else {
							// do something with the decoded content
							body = decoded_content
						}
					default:
						// Data is not encoded, do something with part_data
						body = part
			}
			// deal with media type
			switch {
				case strings.Contains(mediaType, "text/html"):
					html = string(body)
				case strings.Contains(mediaType, "text/plain"):
					text = string(body)
			}
		}
	}
	return
}

