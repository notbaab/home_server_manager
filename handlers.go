package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jrudio/go-plex-client"
	"github.com/notbaab/plexdibella"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func turnOffHandler(w http.ResponseWriter, r *http.Request) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		cmd := exec.Command("shutdown", "-h", "now")
		runCommand(cmd)
	}()
}

func listDirectoryHandler(w http.ResponseWriter, r *http.Request) {

}

func messageResponder(conn *websocket.Conn, uuid string) chan json.RawMessage {
	var messageChannel = make(chan json.RawMessage)

	go func() {
		// read from the byte channel and broadcast a message with the uuid
		// to anything listening
		data, more := <-messageChannel
		if !more {
			return
		}

		resp := ResponseMessage{UUID: uuid, Data: data}
		conn.WriteJSON(resp)
	}()

	return messageChannel
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func insertPlexLoginData(w http.ResponseWriter, r *http.Request) {
	var p PlexMessage
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	if err := decoder.Decode(&p); err != nil {
		log.Println(err)
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := insertPlexData(DB, p.URL, p.Token); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	fmt.Printf("request %+v\n", p)

	respondWithJSON(w, http.StatusCreated, SimpleMessage{"Success"})
}

func websocketApiHandler(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	for {
		var message Message
		err := conn.ReadJSON(&message)
		if err != nil {
			log.Println("Error, exiting websocket loop")
			log.Println(err)
			return
		}

		doAction(message, conn)
	}
}

func runPlexCleanup(messageChan chan json.RawMessage) {
	defer close(messageChan)

	info := Info{Msg: "starting"}
	data, err := json.Marshal(&info)
	messageChan <- data

	// plexMessage := makePlexMessage(message)
	p, err := plex.New("http://data", "blah")

	renameMap, err := plexdibella.GetAllCleanNames(p)

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(renameMap)
}

func doAction(message Message, conn *websocket.Conn) error {
	response := &Message{}
	response.Action = "resp"

	switch message.Action {
	case "turn-off":
		go func() {
			time.Sleep(100 * time.Millisecond)
			cmd := exec.Command("shutdown", "-h", "now")
			runCommand(cmd)
		}()
		break
	case "list-dir":
		dir := getDirFromRenameMessage(message)
		subDirs, files := getDirContents(dir)
		dirList := DirectoryContentsMessage{Files: files, Dirs: subDirs}
		response.Args, _ = json.Marshal(&dirList)
		break
	case "set-plex-data":
		break
	case "cleanup":
		messageChan := messageResponder(conn, message.Id)
		go runPlexCleanup(messageChan)
		break
	}

	return nil
}
