package internal



import (
	"fmt"
	"math/rand"
	"os"
	"log"
	"net"
	"time"
	"sync"
)



// Server is the state machine for the Raft consensus algorithm
// Implements single node consensus.
// raft.Consensus has *server as an embedded field which provides access to the server's methods.
type Server struct {

	mu sync.Mutex

	serverId int
	peerIds []int

	cm *Consensus
	rpcProxy *RPCProxy

	rpcServer *RPCServer
	listener net.Listener

	ready <-chan interface{}
	quit chan interface{}
	wg sync.WaitGroup
	
}

func (s *Server) NewServer() {
	s := new(Server)
	
	s.wg.Add(1)
	go s.serve()
}