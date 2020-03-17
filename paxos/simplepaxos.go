package main

import (
	"fmt"
	"sync"
	"time"
)

type value struct {
	nodenum   int
	val       int
	mestype   messagetype
	committed bool
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
)

type node struct {
	name int

	in  chan value
	out []chan value

	promisedvotes int

	desicionVotes int

	promisedval int

	promisednodes []int //This is required to keep track of which nodes have promised so that we can send accept to them only after they send promise, otherwise wince the accep value does not match the promised value the node will ignore and the consensus cant be reached if other nodes are down and this nodes also does not send a decicion

	isPromiseVoteWon bool //This is required so once promise is done after voting in case some node replies with delay then it should not send another accept

	isConsensusVoteWon bool //This is required so that once the consensus is done, dupllicate consensus message is not printed

	state messagetype

	committedValue int
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
	n.desicionVotes = 1 //One vote is of the proposer
	n.isConsensusVoteWon = false
	n.isPromiseVoteWon = false
	n.promisednodes = []int{}
}

func (n *node) executeOnValueReceived(rec value) {
	//fmt.Println("Triggered")
	proposemutex.Lock()
	defer proposemutex.Unlock()

	switch rec.mestype {
	case propose: //this message will only be recieved by the proposer so the proposer flag is not required
		if n.committedValue != 0 {
			rec.val = n.committedValue
			rec.committed = true
		} else if rec.val > n.promisedval {
			n.promisedval = rec.val
			n.state = propose
			n.setdefaults()
		} else if rec.val < n.promisedval {
			//fmt.Println("The Proposed value:", rec.val, " declined")
			break
		}
		//fmt.Println(rec.mestype, ": Node:", n.name, " received from ", rec.nodenum, " value:", rec.val)

		if n.name < rec.nodenum {
			//fmt.Println("Sending ", promise, ": Node:", n.name, " to ", rec.nodenum, " value:", rec.val, " on ", (4*rec.nodenum)+(n.name))
			n.out[(4*rec.nodenum)+(n.name)] <- value{nodenum: n.name, val: rec.val, mestype: promise}
		} else {
			//fmt.Println("Sending ", promise, ": Node:", n.name, " to ", rec.nodenum, " value:", rec.val, " on ", (4*rec.nodenum)+(n.name-1))
			n.out[(4*rec.nodenum)+(n.name-1)] <- value{nodenum: n.name, val: rec.val, mestype: promise}
		}
	case promise:
		if rec.committed {
			n.broadcast(value{val: rec.val, nodenum: n.name, mestype: accept})
			n.broadcast(value{val: rec.val, nodenum: n.name, mestype: desicion})
			break
		}

		if rec.val > n.promisedval {
			n.setdefaults()
			n.promisedval = rec.val
			n.state = promise
		} else if rec.val < n.promisedval {
			break
		}

		n.promisednodes = append(n.promisednodes, rec.nodenum)
		n.promisedvotes = n.promisedvotes + 1

		//fmt.Println(promise, ": Node:", n.name, " from ", rec.nodenum, " value:", rec.val, "WINS ", n.promisedvotes, " VOTES")

		if n.isPromiseVoteWon {
			n.sendtospeficnodes(value{val: rec.val, nodenum: n.name, mestype: accept}, rec.nodenum)
			break
		}

		//fmt.Println(promise, ": Node:", n.name, " from ", rec.nodenum, " value:", rec.val, "WINS ", n.promisedvotes, " Winning VOTES")

		n.state = promise
		if n.promisedvotes > numofnodes/2 {
			n.isPromiseVoteWon = true
			for _, pn := range n.promisednodes {
				n.sendtospeficnodes(value{val: rec.val, nodenum: n.name, mestype: accept}, pn)
			}
			n.broadcast(value{val: rec.val, nodenum: n.name, mestype: desicion})
		}

	case accept:
		if !n.validateState(rec.mestype) {
			break
		} else if n.promisedval != rec.val {
			break
		}

		fmt.Println("Sending ", desicion, ": Node:", n.name, " from ", rec.nodenum, " value:", rec.val)
		n.state = accept
		n.broadcast(value{val: rec.val, nodenum: n.name, mestype: desicion})

	case desicion:
		if !n.validateState(rec.mestype) {
			//fmt.Println("Rejected:", desicion, " Node:", n.name, " from ", rec.nodenum, " value:", rec.val, " sent state:", rec.mestype, " my state:", n.state)
			break
		} else if n.promisedval != rec.val {
			break
		}

		n.desicionVotes = n.desicionVotes + 1

		if n.isConsensusVoteWon {
			break
		}
		//fmt.Println(desicion, " Node:", n.name, " from ", rec.nodenum, " value:", rec.val, " votes:", n.desicionVotes)
		if n.desicionVotes > numofnodes/2 {
			n.committedValue = rec.val
			n.isConsensusVoteWon = true
			n.committedValue = rec.val
			fmt.Println(desicion, " CONSENSUS Node:", n.name, " from ", rec.nodenum, " value:", rec.val)
		}

	}
}
func (n *node) sendtospeficnodes(val value, targetnode int) {
	//fmt.Println("Sending ", val.mestype, "from ", n.name, "to Node:", targetnode, " for value:", val.val)
	if n.name < targetnode {
		n.out[(4*targetnode)+(n.name)] <- val
	} else {
		n.out[(4*targetnode)+(n.name-1)] <- val
	}
}

func (n *node) validateState(mestype messagetype) bool {
	// if n.name == 1 || n.name == 2 {
	// 	return false
	// }
	// if (n.state == propose && mestype == accept) || (n.state == promise && mestype == accept) {
	// 	return true
	// } else if n.state == accept && mestype == desicion {
	// 	return true
	// }
	// return false
	return true
}

func (n *node) Propose(propVal int) {
	n.broadcast(value{nodenum: n.name, val: propVal, mestype: propose})
}

func (n *node) broadcast(val value) {
	for i := 0; i < numofnodes; i++ {
		if i == n.name {
			continue
		}
		//fmt.Println(val.mestype, ": Node:", n.name, " for ", i)
		if i < n.name {
			n.out[(4*i)+(n.name-1)] <- val
			//fmt.Println(val.mestype, ": Node:", n.name, " for ", i, " on ", (4*i)+(n.name-1), " sent:", val.val)
		} else {
			n.out[(4*i)+n.name] <- val
			//fmt.Println(val.mestype, ": Node:", n.name, " for ", i, " on ", (4*i)+n.name, " sent:", val.val)
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
	go nodes[4].Propose(14)

	//fmt.Println(nodes)
	time.Sleep(duration * 15)
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
