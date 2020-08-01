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

var electmutx sync.Mutex
var trxmutx sync.Mutex
var quorumVotes int

const (
	//proposed  electionstate = "proposed"
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
	quorumVotes = len(nodes) / 2
	fmt.Println(quorumVotes)
	fmt.Println("Peers:", membernodes)

	resetVariables()

	go start()
	time.Sleep(time.Hour * 10)
}

func resetState() {
	st = nodeState{}
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
		fmt.Println("\n\n\n\n\n\n")
		resetVariables()
		resetPromiseVotes()
		resetState()
		diff++
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
		if result == 1 {
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
	if getState() == accepted || getState() == finalized {
		return 0
	}

	if !(compareAndSetTransaction(trxid)) {
		fmt.Println("Cannot propose, already have a higher Leader Candidate")
		return 0
	}
	if st.State != "" {
		return 0
	}

	resetPromiseVotes()
	fmt.Println("Sending Proposal for transaction:", trxid)

	msg := message{
		Transactionid: trxid,
		Messagetype:   proposal,
		From:          portnumber,
		Leader:        portnumber,
	}

	var wg sync.WaitGroup
	wg.Add(len(membernodes))
	for _, v := range membernodes {
		fmt.Printf("Sending Proposal to: %+v \n", v)
		go func(msg message, to string) {
			defer wg.Done()
			resp, status := postData(msg, to, "propose")
			if status == 200 {
				fmt.Printf("received yes vote for proposal, response %+v \n", resp)
				promiseHist.proposalVoters = append(promiseHist.proposalVoters, resp.From)
				promiseHist.votes++
			} else {
				fmt.Printf("received no vote for proposal, response %+v \n", resp)
			}
		}(msg, v)
	}
	wg.Wait()

	mutx.Lock()
	defer mutx.Unlock()
	if trxid == getTransaction() {
		if promiseHist.votes >= quorumVotes {
			if !(compareAndSet("", accepted) || compareAndSet(promised, accepted)) {
				fmt.Println("received max promise votes but state is,", getState())
				return 0
			}
			fmt.Println("received max promise votes from,", promiseHist.proposalVoters, "I can be the leader")
			return 1
		} else {
			fmt.Println("Received only", promiseHist.votes, ", I cannot be the leader")
			return 0
		}
	}
	fmt.Println("proposed value overwriiten by higher trx,", getTransaction(), ", I cannot be the leader")
	return 0
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
		Leader:        msg.Leader,
	}

	currentState := getState()
	if currentState == "" || currentState == promised {
		//change state to promised with accepted transaction
		if !(compareAndSetTransaction(msg.Transactionid)) {
			fmt.Println("Rejected Proposal,", msg.Transactionid, " is lower than ", getTransaction())
			w.WriteHeader(406)
		} else {
			compareAndSet("", promised)
			fmt.Printf("Accepting propsal %+v \n", msg)
			w.WriteHeader(200)
		}
		writehttpOutput(respMessage, w)
		return

	}

	if st.State == accepted || st.State == finalized {
		//ignore everything unless leader is lost
		fmt.Println("Rejected Proposal, since status already changed to Accepted or finalized", msg.Transactionid)
		respMessage := message{
			Transactionid: msg.Transactionid,
			Messagetype:   promise,
			From:          portnumber,
			Leader:        msg.Leader,
		}
		w.WriteHeader(406)
		writehttpOutput(respMessage, w)
	}

}

func sendaccept(trxid int) {
	if !(getState() == accepted) {
		return
	}

	fmt.Println("Sending Acceptance")
	msg := message{
		Transactionid: trxid,
		Messagetype:   accept,
		From:          portnumber,
		Leader:        portnumber,
	}

	var wg sync.WaitGroup
	for _, v := range promiseHist.proposalVoters {
		wg.Add(1)
		go func(msg message, to string) {
			defer wg.Done()
			fmt.Println("Sending acceptance messages: ", to)
			resp, status := postData(msg, to, "accept")
			if status == 200 {
				fmt.Printf("received yes vote for accept , transaction: %d, from: %s \n", msg.Transactionid, resp.From)
				promiseHist.acceptVotes = promiseHist.acceptVotes + 1
			} else {
				fmt.Println("Did not get yes vote, status", status)
			}
		}(msg, v)
	}
	wg.Wait()
	fmt.Println("Received accept votes", promiseHist.acceptVotes)
	mutx.Lock()
	defer mutx.Unlock()

	if getTransaction() == msg.Transactionid {
		if promiseHist.acceptVotes >= quorumVotes {
			if compareAndSet(accepted, finalized) {
				IAmTheLeader()
			}
		}
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
	currentState := getState()

	if currentState == finalized {
		fmt.Printf("Finalized rejecting accept: %+v \n", msg)
		w.WriteHeader(406)
		return
	} else if msg.Messagetype == accept && getTransaction() == msg.Transactionid && compareAndSet(promised, accepted) {

		fmt.Printf("Accepted Accept: %+v \n", msg)
		//change state to promised with accepted transaction
		//if msg.Transactionid == st.LeaderCandidateTransaction {

		respMessage := message{
			Transactionid: msg.Transactionid,
			Messagetype:   broadcast,
			From:          portnumber,
			Leader:        msg.Leader,
		}

		// if !(compareAndSet("", accepted)) {
		// 	fmt.Println("Could not change state from", st.State, " to", accepted)
		// 	return
		// } else if !(compareAndSet(promised, accepted)) {
		// 	fmt.Println("Could not change state from", st.State, " to", accepted)
		// 	return
		// }

		insertToMap(acceptedNotifications, msg.Transactionid, msg.From)

		//Send Acceptance To All and forget
		//fmt.Println("Votes for :", msg.Transactionid, ": ", len(acceptedNotifications[msg.Transactionid]))

		//if msg.Messagetype == accept {
		if len(acceptedNotifications[msg.Transactionid]) >= quorumVotes && msg.Transactionid == getTransaction() {
			//Leader is elected
			//st.LeaderCandidateTransaction = msg.Transactionid
			//if msg.Leader != "" {
			//saveTheLeader(msg.Leader)
			//} else {
			saveTheLeader(msg.From)
		}

		//}
		fmt.Printf("Deciding whether to send broadcast, %s, %v \n", msg.Messagetype, membernodes)
		if msg.Messagetype != broadcast {
			for _, v := range membernodes {
				if v == msg.From {
					continue //dont send message back to node you got the accept from
				}
				go func(p string) {
					fmt.Println("Sending Broadcast to", p)
					postData(respMessage, p, "accept")
				}(v)

			}
		}
		//}
		fmt.Printf("Responding to Accept with OK, %v \n", respMessage)
		w.WriteHeader(200)
		writehttpOutput(respMessage, w)
		return
	} else {
		if msg.Messagetype == broadcast {
			fmt.Printf("Received Brodcast for %+v", msg)
			respMessage := message{
				Transactionid: msg.Transactionid,
				Messagetype:   "",
				From:          portnumber,
				Leader:        msg.Leader,
			}
			insertToMap(acceptedNotifications, msg.Transactionid, msg.From)

			if len(acceptedNotifications[msg.Transactionid]) >= quorumVotes && msg.Transactionid == getTransaction() {
				//Leader is elected
				//st.LeaderCandidateTransaction = msg.Transactionid
				//if msg.Leader != "" {
				//saveTheLeader(msg.Leader)
				//} else {
				saveTheLeader(msg.Leader)
			}

			w.WriteHeader(200)
			writehttpOutput(respMessage, w)
			return

		}
		fmt.Println("Did not accept Accept my state:", getState(), ", myTransaction,", getTransaction(), " received ", msg)
		// if msg.Messagetype == broadcast {
		// 	fmt.Printf("Received braodcast message for %+v \n", msg)
		// 	if len(acceptedNotifications[msg.Transactionid]) > len(membernodes)/2 {
		// 		//Leader is elected
		// 		//st.LeaderCandidateTransaction = msg.Transactionid

		// 		// st.State = accepted
		// 		// saveTheLeader(msg.Leader)
		// 	}
		// }
		// respMessage := message{}
		// respMessage = message{
		// 	Transactionid: msg.Transactionid,
		// 	Messagetype:   accept,
		// 	From:          portnumber,
		// 	Leader:        msg.Leader,
		// }

		// insertToMap(acceptedNotifications, msg.Transactionid, msg.From)
		// fmt.Println("Votes for :", msg.Transactionid, ": ", len(acceptedNotifications[msg.Transactionid]))

		// if len(acceptedNotifications[msg.Transactionid]) > len(membernodes)/2 {
		// 	//Leader is elected
		// 	st.LeaderCandidateTransaction = msg.Transactionid
		// 	saveTheLeader(msg.Leader)

		// }

		w.WriteHeader(406)
		//writehttpOutput(respMessage, w)
		return
	}

}

func saveTheLeader(from string) {
	fmt.Println("***LEADER ELECTED ITS:", from, "***")
	st.KnowsALeader = true
	st.State = finalized
	st.Leader = from
	resetVariables()
}

func IAmTheLeader() {
	fmt.Println("***I AM THE LEADER***")
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
	if t1.After(t2) || t1.Equal(t2) {
		return 1
	}
	return 0
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

	for _, v := range mp[key1] {
		if v == key2 {
			return
		}
	}

	//mp[key1] = make(map[string]int)
	mp[key1] = append(mp[key1], key2)

	fmt.Println("votes ", mp[key1], ":", len(mp[key1]))
}

func getState() electionstate {
	electmutx.Lock()
	defer electmutx.Unlock()
	return st.State

}

func compareAndSet(oldstate, newState electionstate) bool {
	electmutx.Lock()
	defer electmutx.Unlock()
	if st.State == oldstate {
		st.State = newState
		return true
	}
	return false
}

func getTransaction() int {
	trxmutx.Lock()
	defer trxmutx.Unlock()
	return st.LeaderCandidateTransaction
}

func compareAndSetTransaction(new int) bool {
	trxmutx.Lock()
	defer trxmutx.Unlock()
	if st.LeaderCandidateTransaction == 0 {
		st.LeaderCandidateTransaction = new
		return true
	}

	if compareTransaction(new, st.LeaderCandidateTransaction) == 1 {
		st.LeaderCandidateTransaction = new
		return true
	}
	return false
}
