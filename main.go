package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/api/gmail/v1"
)

const (
	MultipartMixed       = "multipart/mixed"
	MultipartRelated     = "multipart/related"
	MultipartAlternative = "multipart/alternative"
	TextPlain            = "text/plain"
	TextHtml             = "text/html"
)

func main() {
	srv, err := ConnectToGmailApi()
	if err != nil {
		log.Fatalf("Unable to retrieve gmail Client: %v", err)
	}

	var dir string
	fmt.Print("Specify a directory for downloaded emails (absolute path): ")
	fmt.Scan(&dir)

	if dir[len(dir)-1] != '/' {
		dir += "/"
	}

	createDirIfNotExists(dir)

	var msgCount int
	fmt.Print("Enter a number of emails to download: ")
	fmt.Scan(&msgCount)

	before := time.Now()
	fmt.Printf("Started at %v. \n", before)

	for _, msg := range retrieveMessages(srv, "me", msgCount) {
		data := readMessage(msg)
		writeToFile(fullPathOfFile(dir, subjectAndDate(msg)), data)
	}

	after := time.Now()
	fmt.Printf("Finished at %v.\nTook %v.\n", after, after.Sub(before))
}

func retrieveMessages(srv *gmail.Service, userId string, maxCount int) []*gmail.Message {
	log.Printf("Retrieving messages from Gmail API. Max number of results: %v. \n", maxCount)

	response, err := srv.Users.Messages.List(userId).MaxResults(int64(maxCount)).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v.", err)
	}

	var messages []*gmail.Message
	for _, v := range response.Messages {
		msg, err := srv.Users.Messages.Get(userId, v.Id).Do()
		if err != nil {
			log.Fatalf("Error during getting %v message: %v.", v.Id, err)
		}

		messages = append(messages, msg)
	}

	log.Printf("Successful: %d messages were obtained from Gmail API. \n", len(messages))

	return messages
}

func readMessage(msg *gmail.Message) []byte {
	var data []byte

	switch msg.Payload.MimeType {
	case TextHtml, TextPlain:
		var err error
		data, err = base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err != nil {
			log.Fatalf("Error during decoding of %v message: %v.", msg.Id, err)
		}
	case MultipartAlternative:
		for _, v := range msg.Payload.Parts {
			if v.MimeType == TextPlain {
				continue
			}

			decoded, err := base64.URLEncoding.DecodeString(v.Body.Data)
			if err != nil {
				log.Fatalf("Error during decoding of %v message part: %v.", v.PartId, err)
			}

			data = append(data, decoded...)
		}
	case MultipartMixed, MultipartRelated:
		for _, part := range msg.Payload.Parts {
			var decoded []byte
			if part.MimeType == MultipartAlternative {
				var decodedParts []byte
				for _, v := range part.Parts {
					var err error
					decodedParts, err = base64.URLEncoding.DecodeString(v.Body.Data)
					if err != nil {
						log.Fatalf("Error during decoding of %v message part: %v.", v.PartId, err)
					}
				}
				decoded = append(decoded, decodedParts...)
			} else {
				var err error
				decoded, err = base64.URLEncoding.DecodeString(part.Body.Data)
				if err != nil {
					log.Fatalf("Error during decoding of %v message part: %v.", part.PartId, err)
				}
			}

			data = append(data, decoded...)
		}
	}

	return data
}

func writeToFile(filePath string, data []byte) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Error during opening a file: %v.", err)
	}
	defer file.Close()

	n, err := file.Write(data)
	if err != nil {
		log.Fatalf("Error during writing to file: %v.", err)
	}
	log.Printf("Wrote %v bytes to file %q.", n, filePath)
}

func createDirIfNotExists(messagesDir string) {
	if _, err := os.Stat(messagesDir); os.IsNotExist(err) {
		os.MkdirAll(messagesDir, 0700)
	}
}

func fullPathOfFile(directory string, name string) string {
	return fmt.Sprintf("%s%v.html", directory, name)
}

func subjectAndDate(msg *gmail.Message) string {
	var subject string
	var date string
	for _, v := range msg.Payload.Headers {
		if v.Name == "Subject" {
			subject = v.Value
		} else if v.Name == "Date" {
			date = v.Value
		}
	}

	return fmt.Sprintf("%s - %s", subject, date)
}
