package main

import (
	"fmt"
	"sync"
	"time"
)

type value struct {
	nodenum int
	val     int
	mestype messagetype
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
	proposemutex sync.Mutex
	promisemutex sync.Mutex
	commitmutex  sync.Mutex
)

type node struct {
	name int

	in  chan value
	out []chan value

	proposer bool
	votes    int

	desicionVotes int

	promisedval  int
	promisednode int

	promisednodes []int
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
func (n *node) executeOnValueReceived(rec value) {
	//fmt.Println("Triggered")

	switch rec.mestype {
	case propose:
		proposemutex.Lock()
		defer proposemutex.Unlock()

		if rec.val > n.promisedval {
			n.promisedval = rec.val
			n.promisednode = rec.nodenum
			n.votes = 0
			n.proposer = false
			n.promisednodes = []int{}
			n.desicionVotes = 1
			//fmt.Println("Node:", n.name, " Received message from Node:", rec.nodenum, " value:", rec.val)
			//n.out[rec.nodenum] <- value{nodenum: n.name, val: rec.val, mestype: promise}
		} else if rec.val < n.promisedval {
			//n.votes = n.votes + 1
			fmt.Println("The Proposed value:", rec.val, " declined")
		}

		if n.name < rec.nodenum {
			//fmt.Println("Propose: Received message from Node:", rec.nodenum, ", to: ", n.name, " need to send promise on channel: ", (4*rec.nodenum)+(n.name))
			n.out[(4*rec.nodenum)+(n.name)] <- value{nodenum: n.name, val: rec.val, mestype: promise}
		} else {
			//fmt.Println("Propose: Received message from Node:", rec.nodenum, ", to: ", n.name, ", need to send promise on channel: ", (4*rec.nodenum)+(n.name-1))
			n.out[(4*rec.nodenum)+(n.name-1)] <- value{nodenum: n.name, val: rec.val, mestype: promise}
		}

	case promise:
		promisemutex.Lock()
		defer promisemutex.Unlock()
		//fmt.Println("Promised value: ", n.promisedval, " , received value:", rec.val, " , votes:", n.votes)
		if n.proposer {
			if rec.val == n.promisedval {
				n.votes = n.votes + 1
				n.promisednodes = append(n.promisednodes, rec.nodenum)
			}
		}
		if n.votes > numofnodes/2 {
			for _, nodenum := range n.promisednodes {
				fmt.Println("Promise Node:", n.name, ", Received from: ", nodenum)
				if n.name < nodenum {
					//fmt.Println("Promise: Received Promise from Node:", nodenum, ", need to send commit on channel: ", (4*nodenum)+(n.name))
					n.out[(4*nodenum)+(n.name)] <- value{nodenum: n.name, val: rec.val, mestype: accept}
				} else {
					//fmt.Println("Promise: Received message from Node:", rec.nodenum, ", need to send commit on channel: ", (4*nodenum)+(n.name-1))
					n.out[(4*nodenum)+(n.name-1)] <- value{nodenum: n.name, val: rec.val, mestype: accept}
				}
			}
			n.promisednodes = []int{}
		}

	case accept:
		commitmutex.Lock()
		defer commitmutex.Unlock()

		if n.promisedval == rec.val && n.promisednode == rec.nodenum {
			// 	n.promisedval = rec.val
			//fmt.Println("Node: ", n.name, " ,committed:", rec.val)
			if n.name == 1 || n.name == 2 || n.name == 3 {
				break
			}
			n.broadcast(value{val: rec.val, nodenum: n.name, mestype: desicion})
		}
		//fmt.Println("Commit Node:", n.name, ", Received from: ", rec.nodenum, " , Value: ", rec.val)
	case desicion:
		if n.promisedval == rec.val {
			n.desicionVotes = n.desicionVotes + 1
			if n.desicionVotes > numofnodes/2 {
				fmt.Println("Node:", n.name, ", consensus reached for Value: ", rec.val)
				break
			}
			fmt.Println("Node:", n.name, ", no consensus reached for Value: ", rec.val)
		}
	}
}

func (n *node) Propose(propVal int) {
	n.proposer = true
	n.votes = 0
	n.promisedval = propVal
	n.broadcast(value{nodenum: n.name, val: propVal, mestype: propose})
}

func (n *node) broadcast(val value) {
	for i := 0; i < numofnodes; i++ {
		if i == n.name {
			continue
		}
		if i < n.name {
			n.out[(4*i)+(n.name-1)] <- val
			fmt.Println(n.name, " - ", (4*i)+(n.name-1))
		} else {
			n.out[(4*i)+n.name] <- val
			fmt.Println(n.name, " - ", (4*i)+n.name)
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
	fmt.Printf("%d, %d \n", nodes[0].name, nodes[0].votes)
	fmt.Printf("%d, %d \n", nodes[1].name, nodes[1].votes)
	fmt.Printf("%d, %d \n", nodes[2].name, nodes[2].votes)
	fmt.Printf("%d ,%d \n", nodes[3].name, nodes[3].votes)
	fmt.Printf("%d ,%d \n", nodes[4].name, nodes[4].votes)
	time.Sleep(duration * 5)
	go nodes[3].Propose(14)
	time.Sleep(duration * 5)
	fmt.Printf("%d, %d \n", nodes[0].name, nodes[0].votes)
	fmt.Printf("%d, %d \n", nodes[1].name, nodes[1].votes)
	fmt.Printf("%d, %d \n", nodes[2].name, nodes[2].votes)
	fmt.Printf("%d ,%d \n", nodes[3].name, nodes[3].votes)
	fmt.Printf("%d ,%d \n", nodes[4].name, nodes[4].votes)
	time.Sleep(duration * 5)
	go nodes[3].Propose(13)
	time.Sleep(duration * 5)
	fmt.Printf("%d, %d \n", nodes[0].name, nodes[0].votes)
	fmt.Printf("%d, %d \n", nodes[1].name, nodes[1].votes)
	fmt.Printf("%d, %d \n", nodes[2].name, nodes[2].votes)
	fmt.Printf("%d ,%d \n", nodes[3].name, nodes[3].votes)
	fmt.Printf("%d ,%d \n", nodes[4].name, nodes[4].votes)
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
		//	fmt.Println("Creating Node:", i)
		node := &node{
			name:     i,
			out:      chans,
			proposer: false,
			votes:    0,
		}
		node.Read()
		nodes = append(nodes, node)
	}
	return nodes
}
