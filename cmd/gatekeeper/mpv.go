package main

import (
	"os"
	"log"
	"net"
)

type MpvMessage struct {
	track string
}

func startMpvControl(tw *TwitchConn) (chan MpvMessage, bool) {
	mpvIpcAddress := os.Getenv("GATEKEEPER_MPV_IPC_ADDRESS");
	if mpvIpcAddress == "" {
		log.Println("No GATEKEEPER_MPV_IPC_ADDRESS is provided.");
		return nil, false
	}

	msgs := make(chan MpvMessage);

	go func() {
		conn, err := net.Dial("tcp", mpvIpcAddress);

		if err != nil {
			log.Printf("MPV: Could not connect to %s: %s\n", mpvIpcAddress, err);
			return;			// TODO: reconnect on error
		}

		var buf [1024]byte;
		for {
			n, err := conn.Read(buf[:])
			if err != nil {
				log.Printf("MPV: Could not read from %s: %s\n", mpvIpcAddress, err);
				return;
			}

			log.Printf("%s\n", string(buf[:n]))
		}
	}();

	return msgs, true
}
