package main

import (
	"os"
	"log"
	"net"
	"time"
	"encoding/json"
)

type MpvMessage struct {
	track string
}

func SendMessage(tw *TwitchConn, message string) {
	message = ". "+FilterTrailingForbidden(message);
	msg := IrcMsg{Name: IrcCmdPrivmsg, Args: []string{TwitchIrcChannel, message}}
	err := msg.Send(tw.Conn)
	if err != nil {
		log.Println("Error sending Twitch message \"%s\" for channel %s: %s", message, TwitchIrcChannel, err)
	}
}

func connectMpvControl(mpvIpcAddress string, tw *TwitchConn) {
	conn, err := net.Dial("tcp", mpvIpcAddress);
	if err != nil {
		log.Printf("MPV: Could not connect to %s: %s\n", mpvIpcAddress, err);
		return;
	}
	defer conn.Close()

	log.Printf("MPV: Successfully connected to %s", mpvIpcAddress);

	var root interface{}
	decoder := json.NewDecoder(conn)

	for {
		err := decoder.Decode(&root)
		if err != nil {
			log.Printf("MPV: Could not read from %s: %s\n", mpvIpcAddress, err);
			return;
		}
		log.Println(root);
	}
}

func startMpvControl(tw *TwitchConn) (chan MpvMessage, bool) {
	mpvIpcAddress := os.Getenv("GATEKEEPER_MPV_IPC_ADDRESS");
	if mpvIpcAddress == "" {
		log.Println("No GATEKEEPER_MPV_IPC_ADDRESS is provided.");
		return nil, false
	}

	msgs := make(chan MpvMessage);

	go func() {
		for {
			connectMpvControl(mpvIpcAddress, tw);
			time.Sleep(5*time.Second)
		}
	}();

	return msgs, true
}
