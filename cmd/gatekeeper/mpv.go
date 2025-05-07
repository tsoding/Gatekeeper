package main

import (
	"os"
	"log"
	"net"
	"time"
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
		return;			// TODO: reconnect on error
	}

	log.Printf("MPV: Successfully connected to %s", mpvIpcAddress);

	var buf [1024]byte;
	for {
		n, err := conn.Read(buf[:])
		if err != nil {
			log.Printf("MPV: Could not read from %s: %s\n", mpvIpcAddress, err);
			return;
		}

		SendMessage(tw, string(buf[:n]));
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
