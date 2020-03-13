package main

import (
	"fmt"
	"time"
)

type value struct {
	nodenum int
	val     int
	mestype messagetype
}
type messagetype string

const (
	propose messagetype = "propose"
	promise messagetype = "promise"
	commit  messagetype = "commit"
)

const (
	numofnodes = 3
)

var (
	numofchannels int
)

type node struct {
	name int

	in  chan value
	out []chan value

	proposer bool
	votes    int

	promised int
}

func (n *node) Read() {
	go func() {
		for {
			rec := <-n.in
			switch rec.mestype {
			case propose:
				n.promised = rec.val
				n.out[rec.nodenum] <- value{
					nodenum: n.name,
					val:     10,
					mestype: promise,
				}

			case promise:
				if n.proposer {
					n.votes = n.votes + 1
					if n.votes > numofnodes/2 {
						fmt.Println(" Node:", n.name, ", I won the election, with value: ", rec.val)
						n.commit(rec.val)
					}
				}

			case commit:
				fmt.Println("committed")
				if n.promised == rec.val {
					n.promised = rec.val
				}

			}

		}
	}()
}

func (n *node) commit(comVal int) {
	for i := 0; i < numofchannels; i++ {
		if i == n.name {
			continue
		}

		n.out[i] <- value{
			nodenum: 0,
			val:     comVal,
			mestype: commit,
		}
	}
}

func (n *node) Propose(propVal int) {
	n.proposer = true
	n.votes = 0
	for i := 0; i < numofchannels; i++ {
		if i == n.name {
			continue
		}

		n.out[i] <- value{
			nodenum: n.name,
			val:     propVal,
			mestype: propose,
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
	nodes[0].Propose(10)
	nodes[1].Propose(11)
	time.Sleep(duration * 1000)

}

func initialize() []*node {
	chans := []chan value{}
	var nodes []*node
	numofchannels = (factorial(numofnodes) / (factorial(numofnodes-2) * 2))
	for i := 0; i < numofchannels; i++ {
		chans = append(chans, make(chan value))
	}

	for i := 0; i < numofnodes; i++ {
		node := &node{
			name:     i,
			in:       chans[i],
			out:      chans,
			proposer: false,
			votes:    0,
		}
		node.Read()
		nodes = append(nodes, node)
	}
	return nodes
}

func factorial(n int) int {
	if n == 1 {
		return 1
	}
	return n * factorial(n-1)
}
