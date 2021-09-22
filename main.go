package main

import (
	"encoding/json"
	"flag"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"log"
	"net/url"
)

type clientSDPs struct {
	offer webrtc.SessionDescription
	answer webrtc.SessionDescription
}

func sdpToByteslice(sdp webrtc.SessionDescription) ([]byte, error) {
        return json.Marshal(sdp)
}

func bytesliceToSDP(sdpBs []byte) (*webrtc.SessionDescription, error) {
        sdp := &webrtc.SessionDescription{}
        return sdp, json.Unmarshal(sdpBs, sdp)
}


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

func recvMsgWebsocketNonBlocking(conn *websocket.Conn) []byte {
        _, msg, readMsgErr := conn.ReadMessage()

        if readMsgErr != nil {
                panic(readMsgErr)
        }

        return msg
}
func recvMsgWebsocketBlocking(conn *websocket.Conn, done chan *webrtc.SessionDescription) *webrtc.SessionDescription {
	defer close(done)
	_, msg, readMsgErr := conn.ReadMessage()

	if readMsgErr != nil {
		panic(readMsgErr)
	}

	msgSDP, convByteSliceToSDPMsgErr := bytesliceToSDP(msg)

	if convByteSliceToSDPMsgErr != nil {
		panic(convByteSliceToSDPMsgErr)
	}

	done <- msgSDP

	return msgSDP
}


func onPeerConnCloseDetectErr(peerConnection *webrtc.PeerConnection) {
        if cErr := peerConnection.Close(); cErr != nil {
                log.Printf("cannot close peerConnection: %v\n", cErr)
        }
        log.Println("Everything is fine. Closing peer connection.")
}

func execInitiatorsSDPSignaling(c *websocket.Conn, peerConnection *webrtc.PeerConnection) *clientSDPs {
	onPeerConnCloseDetectErr(peerConnection)
	localOffer, createOfferErr := peerConnection.CreateOffer(&webrtc.OfferOptions{})
	//localOffer.Type = 1

	if createOfferErr != nil {
		log.Printf("API: Error creating offer! Content: %+v\n", localOffer)
		panic(createOfferErr)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	errSetLocalDescr := peerConnection.SetLocalDescription(localOffer)
	<-gatherComplete

	if errSetLocalDescr != nil {
		log.Println("API: Error setting local descr (offer)!")
		panic(errSetLocalDescr)
	}

	offerByteSlice, sdpToBytesliceErr := sdpToByteslice(localOffer)

	if sdpToBytesliceErr != nil {
		log.Printf("WebRTCClient: Error setting local descr (offer)! Content: %+v\n", offerByteSlice)
		panic(sdpToBytesliceErr)
	}

	done := make(chan *webrtc.SessionDescription)
	go recvMsgWebsocketBlocking(c, done)
	remoteAnswer := *(<-done)

	log.Printf("Remote answer received (before setting it!): %+v\n", remoteAnswer)

	errSetRemoteDescr := peerConnection.SetRemoteDescription(remoteAnswer)

	if errSetRemoteDescr != nil {
		log.Printf("API: Error setting remote descr (answer)! Content: %+v\n", remoteAnswer)
		panic(errSetRemoteDescr)
	}

	log.Printf("Remote answer received (after setting it!): %+v\n", remoteAnswer)

    return &clientSDPs{
    	offer: localOffer,
    	answer: remoteAnswer,
	}
}
func execReceiversSDPSignaling(c *websocket.Conn, peerConnection *webrtc.PeerConnection) *clientSDPs {
    done := make(chan *webrtc.SessionDescription)
    go recvMsgWebsocketBlocking(c, done)
    remoteOffer := *(<-done)

	onPeerConnCloseDetectErr(peerConnection)

	errSetRemoteDescr := peerConnection.SetRemoteDescription(remoteOffer)

	if errSetRemoteDescr != nil {
		log.Printf("API: Error setting remote descr (offer)! Content: %+v\n", remoteOffer)
		panic(errSetRemoteDescr)
	}

	localAnswer, createAnswerErr := peerConnection.CreateAnswer(&webrtc.AnswerOptions{})
	//localOffer.Type = 1

	if createAnswerErr != nil {
		log.Printf("API: Error creating local answer! Content: %+v\n", localAnswer)
		panic(createAnswerErr)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	errSetLocalAnswer := peerConnection.SetLocalDescription(localAnswer)
	<-gatherComplete

	if errSetLocalAnswer != nil {
		log.Printf("API: Error setting local descr (answer)! Content: %+v\n", localAnswer)
		panic(errSetLocalAnswer)
	}

	localAnswerByteSlice, _ := sdpToByteslice(localAnswer)
	sendMsgWebsocket(c, localAnswerByteSlice)

	return &clientSDPs{
		offer: localAnswer,
		answer: remoteOffer,
	}
}


func main() {
	pIAmInitiator := flag.Bool("initiator", false, "Set initiator mode")
	pIAmReceiver := flag.Bool("receiver", false, "Set receiver mode")
	flag.Parse()

	iAmInitiator := *pIAmInitiator
	iAmReceiver := *pIAmReceiver

	if iAmInitiator && iAmReceiver {
		panic("You must not provide both --initiator and --receiver flag. Aborting. Bye!")
	}

    c := connectToWebsocket()

	peerConnection, newPeerConnErr := webrtc.NewPeerConnection(webrtc.Configuration{})

	if newPeerConnErr != nil {
	    log.Printf("API: Error creating new peer connection! Content: %+v\n", peerConnection)
		panic(newPeerConnErr)
	}

	if iAmInitiator {
		sdps := execInitiatorsSDPSignaling(c, peerConnection)
		log.Printf("Initiator: SDPs: %+v\n", sdps)
	} else if iAmReceiver {
		sdps := execReceiversSDPSignaling(c, peerConnection)
		log.Printf("Receiver: SDPs: %+v\n", sdps)
	} else {
		panic("You must provide the -initiator or -receiver flag to select mode. Receiver always must connect first!")
	}
}


