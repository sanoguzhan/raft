package internal

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

type CMState int

const (
	Follower CMState = iota
	Candidate
	Leader
	Dead
)

type LogEntry struct {
	Term    int
	Command interface{}
}

func (s CMState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	case Dead:
		return "Dead"
	default:
		panic("unreachable")
	}
}

type RaftState struct {
	// currentTerm is the latest term the server has seen
	currentTerm int

	// votedFor is the candidateId that received vote in current term
	votedFor int

	// log is the list of log entries
	log []LogEntry
}

// Consensus is the state machine for the Raft consensus algorithm
// Implements single node consensus.
type Consensus struct {
	// mu protects concurrent access to the fields
	mu sync.Mutex

	// id is the unique identifier for this node
	id int

	// peers is the list of peer node identifiers
	peerIds []int

	// server is the server that owns this consensus instance
	server *Server

	// Persistent state on all servers
	raftState RaftState

	// Ephemeral state on all servers
	electionEventStart time.Time
	state              CMState
}

func (c *Consensus) logging(format string, args ...interface{}) {
	format = fmt.Sprintf("[INFO] %s %d ", time.Now(), c.id) + format
	fmt.Printf(format, args...)
}

// RequestVoteArgs are the arguments for the RequestVote RPC
type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

// RequestVoteReply are the reply for the RequestVote RPC
type RequestVoteReply struct {
	Term    int
	Granted bool
}

// NewConsensus creates a new Consensus instance.
// Ready channel signals the CM that the server is ready to start the election timer.
func (c *Consensus) NewConsensus(
	id int,
	peerIds []int,
	server *Server,
	ready <-chan interface{},
) *Consensus {
	c.id = id
	c.peerIds = peerIds
	c.server = server
	go func() {
		<-ready
		c.mu.Lock()
		c.electionEventStart = time.Now()
		c.mu.Unlock()
		c.runElectionTimer()
	}()
	return c
}

// electionTimeout returns a random election timeout duration.
// Retrieves the timeout value from the environment variable RAFT__CORE__ELECTION_TIMEOUT.
func (c *Consensus) electionTimeout() time.Duration {
	value, exists := os.LookupEnv("RAFT__CORE__ELECTION_TIMEOUT")
	interval := 10
	if exists {
		value, err := strconv.Atoi(value)
		if err != nil {
			fmt.Println("Error in parsing election timeout, using default value of 10ms")
		} else {
			interval = value
		}

	}
	return time.Duration(interval+rand.Intn(interval)) * time.Millisecond
}

// startElection starts a new election.
// Increments the current term and transitions to the candidate state.
// Votes for self and resets the election timer.
func (c *Consensus) startElection() {
	c.state = Candidate
	c.raftState.currentTerm++
	currentTerm := c.raftState.currentTerm
	c.electionEventStart = time.Now()
	c.raftState.votedFor = c.id
	c.logging("starting election for CurrentTerm %d, Candidate %d", currentTerm, c.id)
	votesReceived := 1
	for _, peerId := range c.peerIds {
		go func(peerId int) {
			args := RequestVoteArgs{
				Term:        currentTerm,
				CandidateId: c.id,
			}
			var response RequestVoteReply

			c.logging("sending RequestVote to %d\n", peerId)
			
			if err := c.server.RequestVote(peerId, &args, &response); err != nil {
			}
		}(peerId)
	}

}
func (c *Consensus) runElectionTimer() {
	timeout := c.electionTimeout()
	c.mu.Lock()
	termStarted := c.raftState.currentTerm
	c.mu.Unlock()
	timer := time.NewTimer(10 * time.Millisecond)
	for {
		<-timer.C
		c.mu.Lock()
		if c.state != Candidate && c.state != Follower {
			c.logging("election timer stopped, state is %s\n", c.state)
			c.mu.Unlock()
			return
		}
		if termStarted != c.raftState.currentTerm {
			c.logging("election timer stopped, term changed from %d to %d", termStarted, c.raftState.currentTerm)
			c.mu.Unlock()
			return
		}
		elapsed := time.Since(c.electionEventStart)
		if elapsed >= timeout {
			c.startElection()
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
	}

}
