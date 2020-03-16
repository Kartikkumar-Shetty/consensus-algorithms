package main

import (
	"fmt"
	"sync"
	"time"
)

type value struct {
	nodenum         int
	val             int
	mestype         messagetype
	alreadycommited bool
}
type messagetype string

const (
	propose  messagetype = "propose"
	promise  messagetype = "promise"
	accept   messagetype = "accept"
	desicion messagetype = "desicion"
)

const (
	numofnodes    = 5
	numofchannels = 25
)

var (
	proposemutex  sync.Mutex
	promisemutex  sync.Mutex
	commitmutex   sync.Mutex
	desicionmutex sync.Mutex

	setpromisedvaluemutex sync.Mutex
)

type node struct {
	name int

	in  chan value
	out []chan value

	promisedvotes int

	desicionVotes int
	desicionnodes []int

	promisedval  int
	promisednode int

	promisednodes   []int
	temppromiseflag bool //This is required so once promise is done after voting in case some node replies with delay then it should not send another accept

	tempconsensusflag bool //This is required so that once the consensus is done, dupllicate consensus message is not printed

	committedVal int
}

func (n *node) Read() {
	go func() {
		for {
			var rec value
			select {
			case rec = <-n.out[((numofnodes - 1) * n.name)]:
				go n.executeOnValueReceived(rec)
			case rec = <-n.out[((numofnodes-1)*n.name)+1]:
				go n.executeOnValueReceived(rec)
			case rec = <-n.out[((numofnodes-1)*n.name)+2]:
				go n.executeOnValueReceived(rec)
			case rec = <-n.out[((numofnodes-1)*n.name)+3]:
				go n.executeOnValueReceived(rec)
			}

		}
	}()
}

func (n *node) setdefaults() {
	n.promisedvotes = 0
	n.promisednodes = []int{}
	n.desicionVotes = 1 //One vote is of the proposer
	n.tempconsensusflag = false
	n.temppromiseflag = false
}

func (n *node) setPromisedValue(v int) {
	setpromisedvaluemutex.Lock()
	defer setpromisedvaluemutex.Unlock()
	n.promisedval = v
}

func (n *node) executeOnValueReceived(rec value) {
	//fmt.Println("Triggered")

	switch rec.mestype {
	case propose:
		proposemutex.Lock()
		defer proposemutex.Unlock()
		if n.committedVal != 0 {
			fmt.Println("Node:", n.name, " Already committed value: ", n.committedVal)
			if n.name < rec.nodenum {
				//fmt.Println("Propose: Received message from Node:", rec.nodenum, ", to: ", n.name, " need to send promise on channel: ", (4*rec.nodenum)+(n.name))
				n.out[(4*rec.nodenum)+(n.name)] <- value{nodenum: n.name, val: n.committedVal, mestype: promise, alreadycommited: true}
			} else {
				//fmt.Println("Propose: Received message from Node:", rec.nodenum, ", to: ", n.name, ", need to send promise on channel: ", (4*rec.nodenum)+(n.name-1))
				n.out[(4*rec.nodenum)+(n.name-1)] <- value{nodenum: n.name, val: n.committedVal, mestype: promise, alreadycommited: true}
			}
			n.promisedvotes = 0
			n.temppromiseflag = false
			n.desicionVotes = 0
			n.tempconsensusflag = false
			break
		}

		if rec.val > n.promisedval {
			//n.promisedval = rec.val
			n.setPromisedValue(rec.val)
			n.promisednode = rec.nodenum
			n.setdefaults()
		} else if rec.val < n.promisedval {
			fmt.Println("The Proposed value:", rec.val, " declined")

		}

		if n.name < rec.nodenum {
			//fmt.Println("Propose: Received message from Node:", rec.nodenum, ", to: ", n.name, " need to send promise on channel: ", (4*rec.nodenum)+(n.name))
			n.out[(4*rec.nodenum)+(n.name)] <- value{nodenum: n.name, val: rec.val, mestype: promise}
		} else {
			//fmt.Println("Propose: Received message from Node:", rec.nodenum, ", to: ", n.name, ", need to send promise on channel: ", (4*rec.nodenum)+(n.name-1))
			n.out[(4*rec.nodenum)+(n.name-1)] <- value{nodenum: n.name, val: rec.val, mestype: promise}
		}

	case promise: //this message will only be recieved by the proposer so the proposer flag is not required
		promisemutex.Lock()
		defer promisemutex.Unlock()
		//fmt.Println("Promised value: ", n.promisedval, " , received value:", rec.val, " , votes:", n.votes)
		if rec.alreadycommited {
			n.setPromisedValue(rec.val)
			n.broadcast(value{val: rec.val, nodenum: n.name, mestype: accept})
			n.broadcast(value{val: rec.val, nodenum: n.name, mestype: desicion})
			n.promisedvotes = 0
			n.temppromiseflag = false
			n.desicionVotes = 0
			n.tempconsensusflag = false
			break
		} else {

			if rec.val > n.promisedval {
				// make both the flags false in case a new higher value has arrived, sp that a new promise and desicion can be made and all votes can be reset to 0
				//n.promisedval = rec.val
				n.setPromisedValue(rec.val)
				n.setdefaults()

			}

			if rec.val == n.promisedval {
				n.promisedvotes = n.promisedvotes + 1
				n.promisednodes = append(n.promisednodes, rec.nodenum)
			} else {
				//This will reduce the unncecssary logs and bugs
				break
			}

			fmt.Println("Node: ", n.name, " PROMISE from: ", rec.nodenum, " , received value:", rec.val, n.promisedval, n.temppromiseflag, n.promisedvotes)
			if n.temppromiseflag {
				return
			}
			if n.promisedvotes > numofnodes/2 {
				n.broadcast(value{val: rec.val, nodenum: n.name, mestype: accept})
				n.broadcast(value{val: rec.val, nodenum: n.name, mestype: desicion})
				n.promisednodes = []int{}
				n.temppromiseflag = true
			}
		}

	case accept:
		commitmutex.Lock()
		defer commitmutex.Unlock()
		if rec.alreadycommited {
			n.broadcast(value{val: rec.val, nodenum: n.name, mestype: desicion, alreadycommited: true})
			break
		}
		if n.name == 1 || n.name == 2 {
			break
		}
		if n.promisedval == rec.val {
			fmt.Println("Node: ", n.name, " Received Accept:", rec.val)
			n.broadcast(value{val: rec.val, nodenum: n.name, mestype: desicion})
		}

	case desicion:
		desicionmutex.Lock()
		defer desicionmutex.Unlock()
		if n.promisedval == rec.val {
			fmt.Println("Node:", n.name, " Received desicion for Value: ", rec.val, ", from: ", rec.nodenum)
			if n.tempconsensusflag {
				return
			}
			n.desicionVotes = n.desicionVotes + 1
			n.desicionnodes = append(n.desicionnodes, rec.nodenum)
			if n.desicionVotes > numofnodes/2 {
				fmt.Println("Node:", n.name, " CONSENSUS REACHED for Value: ", rec.val, ", nodes: ", n.desicionnodes)
				n.desicionnodes = []int{}
				n.desicionVotes = 0
				n.tempconsensusflag = true
				n.committedVal = rec.val

				break
			}
			//fmt.Println("Node:", n.name, ", received message from:", rec.nodenum, " no consensus reached for Value: ", rec.val)
		}
	}
}

func (n *node) Propose(propVal int) {
	n.broadcast(value{nodenum: n.name, val: propVal, mestype: propose})
}

func (n *node) broadcast(val value) {
	for i := 0; i < numofnodes; i++ {
		if i == n.name {
			continue
		}
		if i < n.name {
			n.out[(4*i)+(n.name-1)] <- val
			fmt.Println("Node:", n.name, " sent:", val.val, " for ", val.mestype, " to ", i)
		} else {
			n.out[(4*i)+n.name] <- val
			fmt.Println("Node:", n.name, " sent:", val.val, " for ", val.mestype, " to ", i)
		}
	}
}

const (
	proposer string = "propser"
	accepter string = "learner"
)

func main() {
	duration := time.Second
	fmt.Println("started...")
	nodes := initialize()
	fmt.Println("num of channels:", numofchannels)

	go nodes[0].Propose(10)
	go nodes[1].Propose(11)
	go nodes[2].Propose(12)
	go nodes[3].Propose(13)

	//fmt.Println(nodes)
	time.Sleep(duration * 5)
	fmt.Printf("%d, %d \n", nodes[0].name, nodes[0].promisedvotes)
	fmt.Printf("%d, %d \n", nodes[1].name, nodes[1].promisedvotes)
	fmt.Printf("%d, %d \n", nodes[2].name, nodes[2].promisedvotes)
	fmt.Printf("%d ,%d \n", nodes[3].name, nodes[3].promisedvotes)
	fmt.Printf("%d ,%d \n", nodes[4].name, nodes[4].promisedvotes)
	time.Sleep(duration * 5)
	go nodes[3].Propose(14)
	time.Sleep(duration * 5)
	fmt.Printf("%d, %d \n", nodes[0].name, nodes[0].promisedvotes)
	fmt.Printf("%d, %d \n", nodes[1].name, nodes[1].promisedvotes)
	fmt.Printf("%d, %d \n", nodes[2].name, nodes[2].promisedvotes)
	fmt.Printf("%d ,%d \n", nodes[3].name, nodes[3].promisedvotes)
	fmt.Printf("%d ,%d \n", nodes[4].name, nodes[4].promisedvotes)
	time.Sleep(duration * 5)
	go nodes[3].Propose(13)
	time.Sleep(duration * 5)
	fmt.Printf("%d, %d \n", nodes[0].name, nodes[0].promisedvotes)
	fmt.Printf("%d, %d \n", nodes[1].name, nodes[1].promisedvotes)
	fmt.Printf("%d, %d \n", nodes[2].name, nodes[2].promisedvotes)
	fmt.Printf("%d ,%d \n", nodes[3].name, nodes[3].promisedvotes)
	fmt.Printf("%d ,%d \n", nodes[4].name, nodes[4].promisedvotes)
	time.Sleep(duration * 5)
	time.Sleep(duration * 1000)

}

func initialize() []*node {
	chans := []chan value{}
	var nodes []*node
	for i := 0; i < numofchannels*(numofchannels); i++ {
		chans = append(chans, make(chan value))
	}

	for i := 0; i < numofnodes; i++ {
		node := &node{
			name: i,
			out:  chans,
		}
		node.Read()
		nodes = append(nodes, node)
	}
	return nodes
}
