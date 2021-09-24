package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"log"
	"net/url"
)

type msgType int

const (
	// SDPOffer = 1, SDPAnswer = 2, ICECandidate = 3
    SDPOffer msgType = iota + 1
	SDPAnswer
    ICECandidate
)

type msgWrapper struct {
	msg   []byte
	_type msgType
}

func (m msgType) String() string {
	return [...]string{"SDPOffer", "SDPAnswer", "ICECandidate"}[m-1]
}

func (m msgType) EnumIndex() int {
	return int(m)
}

type clientSDPs struct {
	offer  *webrtc.SessionDescription
	answer *webrtc.SessionDescription
}

func sdpToByteslice(sdp webrtc.SessionDescription) ([]byte, error) {
	return json.Marshal(sdp)
}

func bytesliceToSDP(sdpBs []byte) (*webrtc.SessionDescription, error) {
	sdp := &webrtc.SessionDescription{}
	return sdp, json.Unmarshal(sdpBs, sdp)
}

func connectToWebsocket() *websocket.Conn {
	url := url.URL{
		Scheme: "ws",
		Host:   "127.0.0.1:8891",
		Path:   "/ws",
	}

	conn, _, defDialErr := websocket.DefaultDialer.Dial(url.String(), nil)

	if defDialErr != nil {
		log.Println("Error calling websocket.DefaultDialer.Dial()")
		panic(defDialErr)
	}
	return conn
}

func sendMsgWebsocket(conn *websocket.Conn, msg []byte, mtype msgType) *msgWrapper {
	log.Printf("sendMsgWebsocket(): MsgType passed: %+v\n", mtype)

	if mtype < 1 || msg == nil {
		log.Fatal("sendMsgWebsocket(): MsgTyp invalid or empty message. Not sending anything.")
	}

	msgwrapper := &msgWrapper{
		msg: msg,
		_type: mtype,
	}
    log.Printf("sendMsgWebsocket(): MsgWrapper constructed: %+v\n", *msgwrapper)

	msgtypeBs, marshalErr := json.Marshal(msgwrapper)

	if marshalErr != nil {
		log.Fatalf("sendMsgWebsocket: Marshalling Go struct to JSON failed: %+v\n", marshalErr)
	}

	writeMsgErr := conn.WriteMessage(websocket.TextMessage, msgtypeBs)

	if writeMsgErr != nil {
		panic(writeMsgErr)
	}

	return msgwrapper
}

func recvMsgWebsocketBlocking(conn *websocket.Conn, done chan *msgWrapper) {
	defer close(done)

	_, msg, readMsgErr := conn.ReadMessage()

	if readMsgErr != nil {
		panic(readMsgErr)
	}

	msgwrapper := &msgWrapper{}
	json.Unmarshal(msg, msgwrapper)

	done <- msgwrapper
}

func execInitiatorsSDPSignaling(c *websocket.Conn, peerConnection *webrtc.PeerConnection) *clientSDPs {
	localOffer, createOfferErr := peerConnection.CreateOffer(&webrtc.OfferOptions{})

	if createOfferErr != nil {
		log.Printf("API: Error creating offer! Content: %+v\n", localOffer)
		panic(createOfferErr)
	}

	errSetLocalDescr := peerConnection.SetLocalDescription(localOffer)

	if errSetLocalDescr != nil {
		log.Println("API: Error setting local descr (offer)!")
		panic(errSetLocalDescr)
	}

	offerByteSlice, sdpToBytesliceErr := sdpToByteslice(*peerConnection.LocalDescription())

	if sdpToBytesliceErr != nil {
		log.Printf("WebRTCClient: Error setting local descr (offer)! Content: %+v\n", offerByteSlice)
		panic(sdpToBytesliceErr)
	}

	log.Printf("Initiator: Will send my initial raw SDPOffer now. MsgType should be 1: %v\n", int(SDPOffer))
	sendMsgWebsocket(c, offerByteSlice, SDPOffer)

	// Handling OnICECandidate event (Trickle ICE)
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			reqBodyBytes := new(bytes.Buffer)
			json.NewEncoder(reqBodyBytes).Encode(candidate)

			messageBytes := reqBodyBytes.Bytes()
			sendMsgWebsocket(c, messageBytes, ICECandidate)
		}
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Receiver: ICE Connection State has changed to %s \n", connectionState.String())
	})

	done := make(chan *msgWrapper)
	go recvMsgWebsocketBlocking(c, done)
	remoteAnswer := *(<-done)

	var remoteAnswerSDP *webrtc.SessionDescription

	if remoteAnswer._type == 2 {
		remoteAnswerSDP, _ = bytesliceToSDP(remoteAnswer.msg)
	}

	log.Printf("Receiver: Remote answer received (before setting it!): %+v\n", remoteAnswer)

	var errSetRemoteDescr error

	if remoteAnswerSDP != nil {
		var sdpRemoteAnswer *webrtc.SessionDescription
		sdpRemoteAnswer = remoteAnswerSDP
		errSetRemoteDescr = peerConnection.SetRemoteDescription(*sdpRemoteAnswer)
	} else {
		log.Fatal("Receiver: Remote answer from receiver was empty. Bye!")
	}

	if errSetRemoteDescr != nil {
		log.Printf("API: Error setting remote descr (answer)! Content: %+v\n", remoteAnswer)
		panic(errSetRemoteDescr)
	}

	log.Printf("Remote answer received (after setting it!): %+v\n", remoteAnswer)

	return &clientSDPs{
		offer:  peerConnection.LocalDescription(),
		answer: peerConnection.RemoteDescription(),
	}
}

func execReceiversSDPSignaling(c *websocket.Conn, peerConnection *webrtc.PeerConnection) *clientSDPs {
	done := make(chan *msgWrapper)
	go recvMsgWebsocketBlocking(c, done)
	remoteOffer := *(<-done)

	log.Printf("Receiver: Remote offer from initiator right after reading it: %+v\n", remoteOffer)

	var remoteOfferSDP *webrtc.SessionDescription

	if remoteOffer._type == 1 {
		remoteOfferSDP, _ = bytesliceToSDP(remoteOffer.msg)
	}

	var errSetRemoteDescr error

	if remoteOfferSDP != nil {
		var sdpRemoteOffer *webrtc.SessionDescription
		sdpRemoteOffer = remoteOfferSDP
		errSetRemoteDescr = peerConnection.SetRemoteDescription(*sdpRemoteOffer)
	} else {
		log.Fatal("Remote offer from initiator was empty. Bye!")
	}

	if errSetRemoteDescr != nil {
		log.Printf("API: Error setting remote descr (offer)! Content: %+v\n", remoteOffer)
		panic(errSetRemoteDescr)
	}

	localAnswer, createAnswerErr := peerConnection.CreateAnswer(&webrtc.AnswerOptions{})

	if createAnswerErr != nil {
		log.Printf("API: Error creating local answer! Content: %+v\n", localAnswer)
		panic(createAnswerErr)
	}

	// Handling OnICECandidate event
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			reqBodyBytes := new(bytes.Buffer)
			json.NewEncoder(reqBodyBytes).Encode(candidate)

			messageBytes := reqBodyBytes.Bytes()
			sendMsgWebsocket(c, messageBytes, ICECandidate)
		}
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed to %s \n", connectionState.String())
	})

	errSetLocalAnswer := peerConnection.SetLocalDescription(localAnswer)

	if errSetLocalAnswer != nil {
		log.Printf("API: Error setting local descr (answer)! Content: %+v\n", localAnswer)
		panic(errSetLocalAnswer)
	}

	localAnswerByteSlice, _ := sdpToByteslice(localAnswer)
	sendMsgWebsocket(c, localAnswerByteSlice, SDPAnswer)

	return &clientSDPs{
		offer:  peerConnection.LocalDescription(),
		answer: peerConnection.RemoteDescription(),
	}
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
