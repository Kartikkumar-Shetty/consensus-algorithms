package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type electionstate string
type messagetype string

var mutx sync.Mutex

const (
	proposed  electionstate = "proposed"
	promised  electionstate = "promised"
	accepted  electionstate = "accepted"
	finalized electionstate = "finalized"
)

const (
	proposal  messagetype = "proposal"
	promise   messagetype = "promise"
	accept    messagetype = "accept"
	broadcast messagetype = "broadcast"
)

type message struct {
	Transactionid int         `json:"transactionid"`
	Messagetype   messagetype `json:"messagetype"`
	From          string      `json:"from"`
	Leader        string      `json:"leader"`
}

type nodeState struct {
	State                      electionstate
	IsALeader                  bool
	KnowsALeader               bool
	Value                      string
	LeaderCandidateTransaction int
	Leader                     string
}

var starttime time.Time
var diff int

var st nodeState

var membernodes []string
var portnumber string

type promiseHistory struct {
	votes          int
	proposalVoters []string
	acceptVotes    int
}

var acceptedNotifications map[int][]string // trnsactionid , vote
var promiseHist promiseHistory

func main() {
	starttime = time.Now()
	diff = 0

	reader := bufio.NewReader(os.Stdin)
	port, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	go createHandlers()
	nodes := [5]string{"8081", "8082", "8083", "8084", "8085"}

	portnumber = strings.Trim(strings.Trim(port, "\n"), " ")

	for i := 0; i < len(nodes); i++ {
		if nodes[i] == portnumber {
			continue
		}
		membernodes = append(membernodes, nodes[i])
	}
	fmt.Println("Peers:", membernodes)

	resetVariables()

	go start()
	time.Sleep(time.Hour * 10)
}

func resetVariables() {
	resetPromiseVotes()
	acceptedNotifications = make(map[int][]string)

}
func resetPromiseVotes() {
	promiseHist = promiseHistory{}
}

func start() {
	for {
		if !(time.Now().Minute()-starttime.Minute() > diff) {
			continue
		}
		diff++
		fmt.Println("sleeping")
		//time.Sleep(time.Second * time.Duration(20))
		if st.IsALeader || st.KnowsALeader {
			fmt.Println("Leader is ", st.Leader)
			time.Sleep(time.Second * 5)
			continue
		}
		fmt.Println("Not Leader, Dont Know a Leader")
		random := rand.Intn(100)
		trxid := time.Now().Add(time.Duration(random) * time.Millisecond).Nanosecond()

		result := propose(trxid)
		if result == 1 && !st.KnowsALeader {
			sendaccept(trxid)
		}
	}
}

func createHandlers() {
	fmt.Println("Starting Server at ", portnumber)
	http.HandleFunc("/propose", handleProposal)
	http.HandleFunc("/accept", handleAcceptance)
	err := http.ListenAndServe(fmt.Sprintf(":%s", portnumber), nil)
	panic(err)
}

func propose(trxid int) int {
	resetPromiseVotes()
	fmt.Println("Sending Proposal for transaction:", trxid)
	if compareTransaction(st.LeaderCandidateTransaction, trxid) == 1 {
		return 0
	}

	msg := message{
		Transactionid: trxid,
		Messagetype:   proposal,
		From:          portnumber,
	}

	var wg sync.WaitGroup
	wg.Add(len(membernodes))
	for _, v := range membernodes {
		fmt.Println("Sending Proposal to:", v)
		go func(msg message, to string) {
			defer wg.Done()
			resp, status := postData(msg, to, "propose")
			if status == 200 {
				promiseHist.proposalVoters = append(promiseHist.proposalVoters, resp.From)
				promiseHist.votes++
			}
		}(msg, v)

	}
	wg.Wait()
	if promiseHist.votes > len(membernodes)/2 {
		fmt.Println("received max promise votes from,", promiseHist.proposalVoters)
		return 1
	}
	return 0
}

func sendaccept(trxid int) {
	fmt.Println("Sending Acceptance")
	msg := message{
		Transactionid: trxid,
		Messagetype:   accept,
		From:          portnumber,
	}

	var wg sync.WaitGroup
	for _, v := range promiseHist.proposalVoters {
		wg.Add(1)
		go func(msg message, to string) {
			defer wg.Done()
			fmt.Println("Sending acceptance messages: ", to)
			_, status := postData(msg, to, "accept")
			if status == 200 {
				promiseHist.acceptVotes = promiseHist.acceptVotes + 1
			} else {
				fmt.Println("Did not get yes vote, status", status)
			}
		}(msg, v)
	}
	wg.Wait()
	fmt.Println("Received accept votes", promiseHist.acceptVotes)
	if len(membernodes)/2 < promiseHist.acceptVotes {
		IAmTheLeader()
	}
}

func handleProposal(w http.ResponseWriter, r *http.Request) {
	mutx.Lock()
	defer mutx.Unlock()
	reqdata, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	msg := message{}
	err = json.Unmarshal(reqdata, &msg)
	if err != nil {
		panic(err)
	}

	respMessage := message{
		Transactionid: msg.Transactionid,
		Messagetype:   promise,
		From:          portnumber,
	}
	fmt.Println("Got a proposal, mystate", st.State)
	fmt.Printf("Sending Reply, %+v \n", respMessage)
	if st.State == "" {
		//change state to promised with accepted transaction
		st.State = promised
		st.LeaderCandidateTransaction = msg.Transactionid
		w.WriteHeader(200)
		writehttpOutput(respMessage, w)
		return

	}
	if st.State == promised {
		//check if trasactionid is greater if yes then change transationid and send promise
		sentTransaction := msg.Transactionid
		if compareTransaction(sentTransaction, st.LeaderCandidateTransaction) == 1 {
			fmt.Println("accepting transaction")
			st.State = promised
			st.LeaderCandidateTransaction = sentTransaction
			w.WriteHeader(200)
			writehttpOutput(respMessage, w)
			return

		} else {
			fmt.Println("rejecting transaction")
			respMessage := message{
				Transactionid: msg.Transactionid,
				Messagetype:   "",
				From:          portnumber,
			}
			w.WriteHeader(406)
			writehttpOutput(respMessage, w)
			return
		}
		//if transactionid is less then ignore
	}
	if st.State == accepted || st.State == finalized {
		//ignore everything unless leader is lost
		respMessage := message{
			Transactionid: msg.Transactionid,
			Messagetype:   "",
			From:          portnumber,
		}
		w.WriteHeader(406)
		writehttpOutput(respMessage, w)
	}

}

func handleAcceptance(w http.ResponseWriter, r *http.Request) {
	mutx.Lock()
	defer mutx.Unlock()
	reqdata, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	msg := message{}
	err = json.Unmarshal(reqdata, &msg)
	if err != nil {
		panic(err)
	}
	if st.State == finalized {
		fmt.Println("Fnalized rejecting acceptance")
		w.WriteHeader(406)
		return
	} else {
		fmt.Println("Received Accept:", msg)
		//change state to promised with accepted transaction
		if msg.Transactionid == st.LeaderCandidateTransaction {
			fmt.Println("Promised and same transaction")
			st.State = accepted

			respMessage := message{
				Transactionid: msg.Transactionid,
				Messagetype:   broadcast,
				From:          portnumber,
			}
			if msg.Leader != "" {
				respMessage.Leader = msg.Leader
			} else {
				respMessage.Leader = msg.From
			}
			insertToMap(acceptedNotifications, msg.Transactionid, msg.From)

			//Send Acceptance To All and forget
			fmt.Println("Votes:", len(acceptedNotifications[msg.Transactionid]))

			if msg.Messagetype == accept {
				//if len(acceptedNotifications[msg.Transactionid]) > len(membernodes)/2 {
				//Leader is elected
				//st.LeaderCandidateTransaction = msg.Transactionid
				//if msg.Leader != "" {
				//saveTheLeader(msg.Leader)
				//} else {
				saveTheLeader(msg.From)
				//}

				//}
				for _, v := range membernodes {
					if v == msg.From {
						continue //dont send message back to node you got the accept from
					}
					go func() {
						postData(respMessage, v, "accept")
					}()

				}
			}
			if msg.Messagetype == broadcast {
				if len(acceptedNotifications[msg.Transactionid]) > len(membernodes)/2 {
					//Leader is elected
					//st.LeaderCandidateTransaction = msg.Transactionid
					if msg.Leader != "" {
						saveTheLeader(msg.Leader)
					} else {
						saveTheLeader(msg.From)
					}

				}
			}
			w.WriteHeader(200)
			writehttpOutput(respMessage, w)
			return
		} else {
			fmt.Println("Promised and different transaction")

			respMessage := message{}
			respMessage = message{
				Transactionid: msg.Transactionid,
				Messagetype:   accept,
				From:          portnumber,
			}
			if msg.Leader != "" {
				respMessage.Leader = msg.Leader
			} else {
				respMessage.Leader = msg.From
			}
			insertToMap(acceptedNotifications, msg.Transactionid, msg.From)
			fmt.Println("Votes:", len(acceptedNotifications[msg.Transactionid]))
			if len(acceptedNotifications[msg.Transactionid]) > len(membernodes)/2 {
				//Leader is elected
				st.LeaderCandidateTransaction = msg.Transactionid
				saveTheLeader(msg.Leader)

			}

			w.WriteHeader(200)
			writehttpOutput(respMessage, w)
			return
		}

	}
}

func saveTheLeader(from string) {
	fmt.Println("Leader Elected its:", from)
	st.KnowsALeader = true
	st.State = finalized
	st.Leader = from
	resetVariables()
}

func IAmTheLeader() {
	fmt.Println("I am the Leader")
	st.KnowsALeader = true
	st.IsALeader = true
	st.State = finalized
	st.Leader = portnumber
	resetVariables()
}

func writehttpOutput(respMessage message, w http.ResponseWriter) {
	respdata, err := json.Marshal(respMessage)
	if err != nil {
		panic("acceptProposal:" + err.Error())
	}
	w.Write(respdata)
}

func compareTransaction(trx1, trx2 int) int {

	t1 := time.Unix(0, int64(trx1))
	t2 := time.Unix(0, int64(trx2))
	if t1.After(t2) {
		return 1
	}
	return 2
}

func postData(msg message, to string, api string) (message, int) {
	data, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	url := fmt.Sprintf("http://localhost:%s/%s", to, api)
	resp, err := http.Post(url, "", bytes.NewReader(data))
	if err != nil {
		panic("postData:" + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 406 {
		panic("postData: Error Response" + resp.Status)
	}

	if resp.StatusCode == 406 {
		return message{}, resp.StatusCode
	}

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic("postData:" + err.Error())
	}

	respMessage := message{}

	err = json.Unmarshal(respData, &respMessage)
	if err != nil {
		panic("postData:" + err.Error())
	}

	//fmt.Println("received message:", respMessage)
	return respMessage, resp.StatusCode
}

func insertToMap(mp map[int][]string, key1 int, key2 string) {
	fmt.Println("key1:", key1, ", key2:", key2)

	//mp[key1] = make(map[string]int)
	mp[key1] = append(mp[key1], key2)

	fmt.Println("votes ", mp[key1], ":", len(mp[key1]))
}
