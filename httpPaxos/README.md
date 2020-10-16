Leader Election in a distributed system can become complicated as the number of nodes in a cluster increases.  
Below is a short article on different cases that I discovered while trying to implement PAXOS to elect a leader with a different number of nodes.  
 
The entire algorithm goes through three states, PREPARE, PROMISE, ACCEPT. Let us take an example of a three-node cluster.  
   
In this example, every Message sent between nodes has two values a UniqueID and NodeID.   
 
The first step is to PREPARE, each node produces a unique ID and proposes to be the leader by broadcasting a PREPARE request with a Unique-ID and NodeID to every other node. In this case, Node1 broadcasts ID (100,1), Node2 broadcasts (99,2) and Node3 broadcasts (101,3), where 99, 100, 101 are UniqueIDs and 1,2,3 are NodeIDs.  
   
Of course, in an ideal situation, the Node which proposes the Unique ID should win. But due to the unpredictable nature of the network connection, this might not be the usual case.   
   
The second step of the algorithm is PROMISE, the nodes that accept the PREPARE request replies with a PROMISE response to the node from which it has received the highest proposed Unique ID. Once Promised the node cannot accept any PREPARE request lower than the accepted ID. For e.g. If Node2 accepts the PREPARE request of Node3 with UniqueID 101, it can not accept a PREPARE request from Node1 with UniqueID 100.  
   
For the proposing node, once it receives PROMISE from the maximum number of nodes in the cluster, it will send an ACCEPT request to all acceptor nodes(i.e. nodes which accepted the PREPARE and responded with PROMISE).  
Once a node accepts an ACCEPT message it cannot change its value for the current round of the election.  
   
The entire election process can unfold in different ways, let's go through three of those cases which would help understand how the process works  
   
Case 1: Node 3 facing network delays  
  
![alt text](https://github.com/Kartikkumar-Shetty/consensus-algorithms/blob/master/httpPaxos/Paxos_1.png)  
  
a.      Node1, Node2 broadcasts PREPARE request to all other nodes  
b.      Node1 rejects Node2's request since Node2's ID 99 is less than Node1's ID 100.  
c.      Node3 rejects Node1's request since Node1's ID 100 is less than Node3's ID 101.  
d.      Node2 sends a promise to Node1 since Node1's ID is greater than Node2's request ID 99.  
e.      Node1 now has promise votes from the maximum number of nodes in the cluster, Node2, and Node1 itself.  
f.      Hence Node1 will now send an ACCEPT-REQ request to Node2, NOTE: Once Node1 moves to ACCEPT state, it will not accept any PREPARE requests.  
g.      Node2 receives the ACCEPT-REQ request from Node1 for ID 100. Since Node2 had promised PREPARE(100,1) it will accept this message and reply with an ACCEPT message and accept Node1 as the leader.  
h.      Node1, even though it sent an ACCEPT-REQ it is not sure whether Node2 will accept it, so it will wait for the broadcast message, once it receives ACCEPT from Node2, it now has two ACCEPT votes, so it will make itself as the leader.  
i.      Node3 now decides to start the leader election with UniqueID 101, and it sends Node1 and Node2 PREPARE requests. But both nodes have already accepted Node1 as the leader and hence they will reply back with Unique ID 100 but NodeID 1 in the PROMISE message.  
j.      Node3 looks at the NodeID value in the PROMISE message and uses that as the NodeID in the rest of the messages, as shown in the diagram, and eventually accepts Node 1 as the leader.  
  
 Case2: Node3 becomes the leader  
   
 ![alt text](https://github.com/Kartikkumar-Shetty/consensus-algorithms/blob/master/httpPaxos/Paxos_2.png)  
   
a.      Node2 sends PREPARE to Node1 and Node2, but both nodes reject it since Node2's Unique ID 99 is lower than Node1's UniqueID 100 and Node2's UniqueID 101  
b.      Node1 and Node2 receive a proposal from Node3 with UniqueID 101 and since the ID of Node3 is greater than both Node1's 100 and Node2' 99, they both PROMISE to Node3.   
c.      Hence the entire cluster has promised to make Node3 the leader.  
d.      The rest of the process is straight forward since both Node1 and Node2 have PROMISED to Node3, Node2 proceeds to ACCEPT and eventually all three nodes accept Node3 as the leader.  
  
Case3: Node1 gets Stuck and has to restart the process  
  
![alt text](https://github.com/Kartikkumar-Shetty/consensus-algorithms/blob/master/httpPaxos/Paxos_2.png)  
  
a.      This is an interesting case, Node3 and Node2 receive a proposal from Node1 with UniqueID 100.  
b.      Node2 replies with PROMISE but Node3 rejects the request.  
c.      Node1 has the maximum number of PROMISE votes(from Node2 and itself) and hence it sends a ACCEPT-REQ message to Node2.  
d.      But before Node2 can receive Node1's ACCEPT-REQ, Node3 has sent a PREPARE request to both Node1 and Node2.  
e.      Node2 replies with a PROMISE but Node1 has already moved on to ACCEPT state and it ignores Node3's PREPARE request.  
f.      Now, Node2 has promised to Node3 and hence it will ignore Node1's ACCEPT-REQ and Node1 will be stuck.  
g.      The election will process with Node3 becoming the leader as shown in the diagram.  
h.      Since Node1 is stuck, it will wait for some random time and restart the election with a higher UniqueID 102.  
i.      But Since Node2 and Node3 have already accepted Node3 as the leader, they will reply back to Node1 with PROMISE message having NodeID-3 as the value.  
j.      Node1 will proceed with the election with NodeID3 and accept NodeID 3 as the leader.  
   
I have written a rough version of the code at this location(I plan to improve this): https://github.com/Kartikkumar-Shetty/consensus-algorithms/tree/master/httpPaxos  
I have tested it and it works three or more nodes and when run the maximum number of nodes will always decide on one leader. Some subset of nodes (which is always less than n/2) will not reach a consensus like in case3, the program needs to be extended so that the remaining nodes will discover the leader when they try for Election the second time.  
