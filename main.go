package main

import (
        "flag"
        "fmt"
        "github.com/pion/webrtc/v3"
        "github.com/gorilla/websocket"
        "log"
        "net/url"
)

func connectToWebsocket() *websocket.Conn  {
        url := url.URL {
                Scheme: "ws",
                Host: "127.0.0.1:8891",
                Path: "/ws",
        }

        conn, _, defDialErr := websocket.DefaultDialer.Dial(url.String(), nil)

        if defDialErr != nil {
                log.Println("Error calling websocket.DefaultDialer.Dial()")
                panic(defDialErr)
        }
        return conn
}

func sendMsgWebsocket(conn *websocket.Conn, msg []byte) []byte {
        writeMsgErr := conn.WriteMessage(websocket.TextMessage, msg)

        if writeMsgErr != nil {
                panic(writeMsgErr)
        }

        return msg
}

func recvMsgWebsocket(conn *websocket.Conn) []byte {
        _, msg, readMsgErr := conn.ReadMessage()

        if readMsgErr != nil {
                panic(readMsgErr)
        }

        return msg
}

func main() {
        pIAmInitiator := flag.Bool("initiator", false, "Set intiator mode")
        pIAmReceiver := flag.Bool("receiver", false, "Set receiver mode")
        flag.Parse()

        iAmInitiator := *pIAmInitiator
        iAmReceiver := *pIAmReceiver

        if iAmInitiator && iAmReceiver {
                panic("You must not provide both --initiator and --receiver flag. Aborting. Bye!")
        }

        peerConnection, errPeerConnection := webrtc.NewPeerConnection(webrtc.Configuration{})

        if errPeerConnection != nil {
                panic(errPeerConnection)
        }

        myOffer, errCreateOffer := peerConnection.CreateOffer(&webrtc.OfferOptions{})

        if errCreateOffer != nil {
                panic(errCreateOffer)
        }

        peerConnection.SetLocalDescription(myOffer)
        myOfferStr := fmt.Sprintf("%s", myOffer)
        myOfferBs := []byte(myOfferStr)

        websocketConn := connectToWebsocket()

        if iAmInitiator {
                log.Println("I am the initiator...")
                // TODO: send myOffer (SDP) to a websocket server on localhost:8889 that relays it to the second client/peer
                log.Println("Send myOffer (SDP) to websocket server that relays it to second client.")
                sentMsg := sendMsgWebsocket(websocketConn, myOfferBs)
                log.Printf("v+%", sentMsg)

                for{}
                // TODO: Listen on websocket server localhost:8889 for offer (SDP)
                log.Println("Listen for response offer.")
        } else {
                log.Println("I am the receiver..")
                // TODO: Listen on websocket server localhost:8889 for initiator's offer (SDP)
                recvdMsg := recvMsgWebsocket(websocketConn)
                log.Printf("v+%", recvdMsg)

                log.Println("Listen for initiator's offer.")
                // TODO: send myOffer (SDP) to a websocket server on localhost:8889 that relays it to the initiator
                log.Println("Send myOffer (SDP) to websocket server that relays it to the initiator.")
        }
}
