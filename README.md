# concordia

## Introduction
Concordia is a file distribution service, users can upload a file to any server in the cluster, 
and the system will deploy this file among all other serves. Paxos consensus algorithm is used as
underlying foundation to provide a strong consistency. The system calculates a file’s MD5 value 
as the value on which consensus must reach, each server tries to find the corresponding file on 
any of other servers. The system provides a http interface for users to upload files, the 
correct MD5 digest must be provided alongside the file for system to verify. Each server uses a 
round-robin method to ask other servers if they have the file this server is looking for.

All values that belong to one dataset uploaded by clients will be put into a sorted log, the system 
makes sure all servers have the exact same log (strong consistency). As long as a client starts a
request to the server, the server will choose a log id for this file and try to broadcast this 
file (actually the digest value as a string) to other servers. This process may fail if there 
are other servers trying to commit the file with the same log id. If a process fails, the 
clients are free to retry it. Therefore the system supports concurrent requests on the same 
dataset, but it might reduce the performance a bit, because it might take longer for a server to 
find a valid log id. A better way may be using consistent hash to distribute requests that belong to 
different datasets to different servers.
 
The quorum number can be configured (from 1 to number of total servers), which means users can set 
different levels of consistency just like DynamoDB. The system supports Byzantine Fault 
Tolerance, basically using PBFT as a guarantee algorithm.
 
(It's quite obvious that Paxos or PBFT is overused to just make a file distribution service, but
this is mainly used as a demo of my dissertation project 😂, so the implementation of Paxos and 
PBFT is important...)
  
## Design
### Paxos
The very purpose of a consensus algorithm is to make sure all nodes in a cluster agree on one
single value (or a list of values that in the same order). Original Paxos algorithm uses a 
two-phase commit method to help different nodes agree on a single value. With some constrains, 
a proposal must be approved by the majority of cluster to be committed. It is very 
flexible: it allows multiple proposers proposing different values concurrently, it allows 
different nodes playing different roles as proposer, acceptor and learner and it tolerates 
nodes crashes and restarts. However high flexibility leads to difficulty in implementation and 
maybe low performance. Therefore there came out other consensus algorithms like Raft to make it
easier in implementation. To my knowledge, Raft is a subset of Paxos, which makes some limits 
and assumptions:
   * only allows one proposer in a limited time span. That strong leader will be in charge of 
   copying values to other servers.
   * the two-phase commit is done during new leader election to make sure the validity of new leader
   * log is only appendable, there should be no gap inside

Therefore, concordia has some features that make the most of advantages of both of those algorithms:
   * One dataset has one log list, concurrent requests are allowed on different nodes to propose 
   values on the same dataset log list. Each node is able to receive new value and propose, it'd 
   be more efficient to use consistency hash to distribute requests to different nodes. There 
   could be some optimisation on how log id is selected.
   * Each proposal originally must go through two-phase commit to be accepted, but this could be 
   optimised to just one phase: after the first time that this node has successfully made a 
   proposal, subsequent proposals could be accepted directly unless there are other proposals 
   from another proposer that have conflicts with this one.
   * There will be no single strong leader, but there should be one steady proposer for one 
   dataset, if that proposer failed, client can retry requests to another node and make that ndoe
   start to propose values for the same dataset. There will be at most some conflicts if the old
   proposer was not actually dead, but the correctness is maintained.
   
Communications between servers are based on RPC, and a self-defined message structure.
   
### Byzantine Fault Tolerance
With digital signature techniques, BFT can be implemented easily. Based on PBFT, concordia 
uses a three-phase commit method to make nodes communicate completely on a proposal to 
determine if it's valid. There actually two main problems to handle:
   1. check wether the value is a true value that clients want to send
   2. check if the value received by nodes are all the same
   
Using digital signature can make it easy to deal with problem 1, and for problem 2 3-phase 
commit method is used to make sure nodes have enough information to draw a conclusion.
     
### Replicated State Machine
   Concordia has a replicated state machine for each dataset, actually it's a log entry list. RSM
   maintains each log entries, knows which entry is committed (ready to be executed) or is being
   proposed. For now concordia searches for an available entry from the start of log list, mark 
   that entry as being proposed. If it turns out that the entry is not successfuly proposed, RSM 
   marks it as free again and it becomes available to be selected next time. There could be 
   some optimisation that different node has its own entry locations (using mod for example) to 
   reduce conflicts.
   
 ### Others
   Servers in concordia uses HTTP to interact with clients. 
    
 
  
  
  
  
  
  
  
  
  
  
  
  
  
  
   
