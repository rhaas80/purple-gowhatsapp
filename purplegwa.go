// package name: purplegwa
package main

/* 
#include <stdint.h>

struct gowhatsapp_message { 
uint64_t timestamp;
char *id;
char *remoteJid; 
char *text;
void *blob;
uint64_t blobsize;
}; 
*/
import "C"

import (
	"encoding/gob"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/Rhymen/go-whatsapp"
)

type waHandler struct{}
type downloadedImageMessage struct {
    msg whatsapp.ImageMessage
    data []byte
}

var textMessages = make(chan whatsapp.TextMessage, 100)
var imageMessages = make(chan downloadedImageMessage, 100)

//export gowhatsapp_go_getMessage
func gowhatsapp_go_getMessage() C.struct_gowhatsapp_message {
        select {
                case message := <- textMessages:
                        return C.struct_gowhatsapp_message{
                                C.uint64_t(message.Info.Timestamp),
                                C.CString(message.Info.Id),
                                C.CString(message.Info.RemoteJid),
                                C.CString(message.Text),
                                nil,
                                0}
                case message := <- imageMessages:
                        return C.struct_gowhatsapp_message{
                                C.uint64_t(message.msg.Info.Timestamp),
                                C.CString(message.msg.Info.Id),
                                C.CString(message.msg.Info.RemoteJid),
                                nil,
                                C.CBytes(message.data),
                                C.uint64_t(len(message.data))} // TODO: rather use size_t?
                default:
                        return C.struct_gowhatsapp_message{
                                C.uint64_t(0),
                                nil,
                                nil,
                                nil,
                                nil,
                                0}
        }
}

func (*waHandler) HandleError(err error) {
        // TODO: propagate disconnect
	fmt.Fprintf(os.Stderr, "gowhatsapp error occoured: %v", err)
}

func (*waHandler) HandleTextMessage(message whatsapp.TextMessage) {
        textMessages <- message
}

func (*waHandler) HandleImageMessage(message whatsapp.ImageMessage) {
        data, err := message.Download()
        if err != nil {
                // TODO: propagate error
                fmt.Printf("gowhatsapp message %v image from %v download failed: %v\n", message.Info.Timestamp, message.Info.RemoteJid, err)
                return
        }
        fmt.Printf("gowhatsapp message %v image from %v size is %d.\n", message.Info.Timestamp, message.Info.RemoteJid, len(data))
        imageMessages <- downloadedImageMessage{message, data}
}

var wac *whatsapp.Conn

//export gowhatsapp_go_login
func gowhatsapp_go_login() bool {
	//create new WhatsApp connection
        wac, err := whatsapp.NewConn(5 * time.Second) // TODO: make timeout user configurable
	if err != nil {
		fmt.Fprintf(os.Stderr, "gowhatsapp error creating connection: %v\n", err)
	} else {
		
		//Add handler
		wac.AddHandler(&waHandler{})
		
                err = login(wac) // TODO: put this into a go routine (return immediately, not blocking the UI during connection), communicate via chan
                // TODO: create qr code, send as image message from something like "login@s.whatsapp.net"
		if err != nil {
			fmt.Fprintf(os.Stderr, "gowhatsapp error logging in: %v\n", err)
		} else {
			return true
		}
	}
	wac = nil
        return false // TODO: forward err instead (for display in frontend)
}

//export gowhatsapp_go_close
func gowhatsapp_go_close() {
        fmt.Fprintf(os.Stderr, "gowhatsapp close()\n")
        wac = nil
}

func login(wac *whatsapp.Conn) error {
	//load saved session
	session, err := readSession()
	if err == nil {
		//restore session
		session, err = wac.RestoreSession(session)
		if err != nil {
			return fmt.Errorf("gowhatsapp restoring failed: %v\n", err)
                        // NOTE: "restore session connection timed out" may indicate phone switched off
		}
	} else {
		return fmt.Errorf("gowhatsapp error during login: no session stored\n")
	}

	//save session
	err = writeSession(session)
	if err != nil {
		return fmt.Errorf("gowhatsapp error saving session: %v\n", err)
	}
	return nil
}

func readSession() (whatsapp.Session, error) {
	session := whatsapp.Session{}
	usr, err := user.Current()
	if err != nil {
		return session, err
	}
	file, err := os.Open(usr.HomeDir + "/.whatsappSession.gob")
	if err != nil {
		return session, err
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&session)
	if err != nil {
		return session, err
	}
	return session, nil
}

func writeSession(session whatsapp.Session) error {
	usr, err := user.Current()
	if err != nil {
		return err
	}
	file, err := os.Create(usr.HomeDir + "/.whatsappSession.gob")
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(session)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	gowhatsapp_go_login()
	<-time.After(1 * time.Minute)
	gowhatsapp_go_close()
}