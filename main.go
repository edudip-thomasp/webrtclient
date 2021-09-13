package main

import (
        "flag"
        "github.com/pion/webrtc/v3"
        "log"
)

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

        if iAmInitiator {
                log.Println("I am the initiator...")
                // TODO: send myOffer (SDP) to a websocket server on localhost:8889 that relays it to the second client/peer
                log.Println("Send myOffer (SDP) to websocket server that relays it to second client.")
                // TODO: Listen on websocket server localhost:8889 for offer (SDP)
                log.Println("Listen for response offer.")
        } else {
                log.Println("I am the receiver..")
                // TODO: Listen on websocket server localhost:8889 for initiator's offer (SDP)
                log.Println("Listen for initiator's offer.")
                // TODO: send myOffer (SDP) to a websocket server on localhost:8889 that relays it to the initiator
                log.Println("Send myOffer (SDP) to websocket server that relays it to the initiator.")
        }
}
