/* player.go
 */

package main

import (
	"container/list"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"net"
	o "orchestra"
	"os"
	"syscall"
	"time"
)

const (
	InitialReconnectDelay = 5 * time.Second
	MaximumReconnectDelay = 300 * time.Second
	ReconnectDelayScale   = 2
	KeepaliveDelay        = 200 * time.Second
	RetryDelay            = 5 * time.Second
)

type NewConnectionInfo struct {
	conn    net.Conn
	timeout time.Duration
}

var (
	ConfigFile     = flag.String("config-file", "/etc/orchestra/player.conf", "Path to the configuration file")
	DontVerifyPeer = flag.Bool("dont-verify-peer", false, "Ignore TLS verification for the peer")
	CertPair       tls.Certificate
	CACertPool     *x509.CertPool
	LocalHostname  string

	receivedMessage     = make(chan *o.WirePkt)
	lostConnection      = make(chan int)
	reloadScores        = make(chan int, 2)
	pendingQueue        = list.New()
	unacknowledgedQueue = list.New()
	newConnection       = make(chan *NewConnectionInfo)
	pendingTaskRequest  = false
	InvalidValueError   = errors.New("Invalid value")
)

func getNextPendingTask() (task *TaskRequest) {
	e := pendingQueue.Front()
	if e != nil {
		task, _ = e.Value.(*TaskRequest)
		pendingQueue.Remove(e)
	}
	return task
}

func appendPendingTask(task *TaskRequest) {
	pendingTaskRequest = false
	pendingQueue.PushBack(task)
}

func getNextUnacknowledgedResponse() (resp *TaskResponse) {
	e := unacknowledgedQueue.Front()
	if e != nil {
		resp, _ = e.Value.(*TaskResponse)
		unacknowledgedQueue.Remove(e)
	}
	return resp
}

func appendUnacknowledgedResponse(resp *TaskResponse) {
	resp.RetryTime = time.Now().Add(RetryDelay)
	unacknowledgedQueue.PushBack(resp)
}

func acknowledgeResponse(jobid uint64) {
	for e := unacknowledgedQueue.Front(); e != nil; e = e.Next() {
		resp := e.Value.(*TaskResponse)
		if resp.id == jobid {
			unacknowledgedQueue.Remove(e)
		}
	}
}

func sendResponse(c net.Conn, resp *TaskResponse) {
	//FIXME: update retry time on Response
	ptr := resp.Encode()
	p, err := o.Encode(ptr)
	o.MightFail(err, "Failed to encode response")
	_, err = p.Send(c)
	if err != nil {
		o.Warn("Transmission error: %s", err)
		c.Close()
		prequeueResponse(resp)
		lostConnection <- 1
	} else {
		appendUnacknowledgedResponse(resp)
	}
}

func prequeueResponse(resp *TaskResponse) {
	unacknowledgedQueue.PushFront(resp)
}

func Reader(conn net.Conn) {
	defer func(l chan int) {
		l <- 1
	}(lostConnection)

	for {
		pkt, err := o.Receive(conn)
		if err != nil {
			o.Warn("Error receiving message: %s", err)
			break
		}
		receivedMessage <- pkt
	}
}

func handleNop(c net.Conn, message interface{}) {
	o.Debug("NOP received")
}

func handleIllegal(c net.Conn, message interface{}) {
	o.Fail("Received illegal message")
}

func handleRequest(c net.Conn, message interface{}) {
	ptr, ok := message.(*o.ProtoTaskRequest)
	if !ok {
		o.Assert("CC stuffed up - handleRequest got something that wasn't a ProtoTaskRequest.")
	}
	task := TaskFromProto(ptr)
	/* search the registry for the task */
	o.Debug("job%d: got request", task.Id)
	existing := TaskGet(task.Id)
	if nil != existing {
		if existing.MyResponse.IsFinished() {
			o.Debug("job%d: resending response...", task.Id)
			sendResponse(c, existing.MyResponse)
		}
	} else {
		// check to see if we have the score
		// add the Job to our Registry
		task.MyResponse = NewTaskResponse()
		task.MyResponse.id = task.Id
		task.MyResponse.State = RESP_PENDING
		TaskAdd(task)
		// and then push it onto the pending job list so we know it needs actioning.
		appendPendingTask(task)
	}
}

func handleAck(c net.Conn, message interface{}) {
	ack, ok := message.(*o.ProtoAcknowledgement)
	if !ok {
		o.Assert("CC stuffed up - handleAck got something that wasn't a ProtoAcknowledgement.")
	}
	if ack.Id != nil {
		acknowledgeResponse(*ack.Id)
	}
}

var dispatcher = map[uint8]func(net.Conn, interface{}){
	o.TypeNop:             handleNop,
	o.TypeTaskRequest:     handleRequest,
	o.TypeAcknowledgement: handleAck,

	/* P->C only messages, should never appear on the wire to us. */
	o.TypeIdentifyClient: handleIllegal,
	o.TypeReadyForTask:   handleIllegal,
	o.TypeTaskResponse:   handleIllegal,
}

func connectMe(initialDelay time.Duration) {
	var backOff time.Duration = initialDelay
	for {
		// Sleep first.
		if backOff > 0 {
			o.Info("Sleeping for %d seconds...", int(backOff.Seconds()))
			time.Sleep(backOff)
			backOff *= ReconnectDelayScale
			if backOff > MaximumReconnectDelay {
				backOff = MaximumReconnectDelay
			}
		} else {
			backOff = InitialReconnectDelay
		}

		tconf := &tls.Config{
			RootCAs: CACertPool,
		}
		tconf.Certificates = append(tconf.Certificates, CertPair)

		// update our local hostname.
		LocalHostname = GetStringOpt("player name")
		if LocalHostname == "" {
			LocalHostname = o.ProbeHostname()
			o.Warn("No hostname provided - probed hostname: %s", LocalHostname)
		}

		masterHostname := GetStringOpt("master")

		raddr := fmt.Sprintf("%s:%d", masterHostname, 2258)
		o.Info("Connecting to %s...", raddr)
		conn, err := tls.Dial("tcp", raddr, tconf)
		if err == nil && !*DontVerifyPeer {
			conn.Handshake()
			err = conn.VerifyHostname(masterHostname)
		}
		if err == nil {
			nc := new(NewConnectionInfo)
			nc.conn = conn
			nc.timeout = backOff
			newConnection <- nc
			return
		}
		o.Warn("Couldn't connect to master: %s", err)
	}
}

func ProcessingLoop() {
	var conn net.Conn
	var nextRetryResp *TaskResponse
	var taskCompletionChan <-chan *TaskResponse
	var connectDelay time.Duration
	var doScoreReload bool

	// kick off a new connection attempt.
	go connectMe(connectDelay)

	// and this is where we spin!
	for {
		var retryDelay time.Duration
		var retryChan <-chan time.Time

		if conn != nil {
			for nextRetryResp == nil {
				nextRetryResp = getNextUnacknowledgedResponse()
				if nil == nextRetryResp {
					break
				}
				retryDelay = nextRetryResp.RetryTime.Sub(time.Now())
				if retryDelay < 0 {
					sendResponse(conn, nextRetryResp)
					nextRetryResp = nil
				}
			}
			if nextRetryResp != nil {
				retryChan = time.After(retryDelay)
			}
		}
		if taskCompletionChan == nil {
			nextTask := getNextPendingTask()
			if nextTask != nil {
				taskCompletionChan = ExecuteTask(nextTask)
			} else {
				if conn != nil && !pendingTaskRequest {
					o.Debug("Requesting tasks...")
					p := o.MakeReadyForTask()
					p.Send(conn)
					pendingTaskRequest = true
				}
			}
		}
		select {
		// Currently executing job finishes.
		case newresp := <-taskCompletionChan:
			o.Debug("job%d: completed with state %s\n", newresp.id, newresp.State)
			// preemptively set a retrytime.
			newresp.RetryTime = time.Now()
			// ENOCONN - sub it in as our next retryresponse, and prepend the old one onto the queue.
			if nil == conn {
				if nil != nextRetryResp {
					prequeueResponse(nextRetryResp)
				}
				nextRetryResp = newresp
			} else {
				o.Debug("job%d: sending response...", newresp.id)
				sendResponse(conn, newresp)
			}
			if doScoreReload {
				o.Info("Reloading scores...")
				LoadScores()
				doScoreReload = false
			}
			taskCompletionChan = nil
		// If the current unacknowledged response needs a retry, send it.
		case <-retryChan:
			sendResponse(conn, nextRetryResp)
			nextRetryResp = nil
		// New connection.  Set up the receiver thread and Introduce ourselves.
		case nci := <-newConnection:
			if conn != nil {
				conn.Close()
			}
			conn = nci.conn
			connectDelay = nci.timeout
			pendingTaskRequest = false

			// start the reader
			go Reader(conn)

			/* Introduce ourself */
			p := o.MakeIdentifyClient(LocalHostname)
			p.Send(conn)
		// Lost connection.  Shut downt he connection.
		case <-lostConnection:
			o.Warn("Lost connection to master")
			conn.Close()
			conn = nil
			// restart the connection attempts
			go connectMe(connectDelay)
		// Message received from master.  Decode and action.
		case p := <-receivedMessage:
			// because the message could possibly be an ACK, push the next retry response back into the queue so acknowledge can find it.
			if nil != nextRetryResp {
				prequeueResponse(nextRetryResp)
				nextRetryResp = nil
			}
			var upkt interface{}
			if p.Length > 0 {
				var err error
				upkt, err = p.Decode()
				o.MightFail(err, "Couldn't decode packet from master")
			}
			handler, exists := dispatcher[p.Type]
			if exists {
				connectDelay = 0
				handler(conn, upkt)
			} else {
				o.Fail("Unhandled packet type %d", p.Type)
			}
		// Reload scores
		case <-reloadScores:
			// fortunately this is actually completely safe as
			// long as nobody's currently executing.
			// who'd have thunk it?
			if taskCompletionChan == nil {
				o.Info("Reloading scores...")
				LoadScores()
			} else {
				o.Info("Deferring score reload (execution in progress)")
				doScoreReload = true
			}
		// Keepalive delay expired.  Send Nop.
		case <-time.After(KeepaliveDelay):
			if conn == nil {
				break
			}
			o.Debug("Sending NOP...")
			p := o.MakeNop()
			p.Send(conn)
		}
	}
}

func main() {
	o.SetLogName("player")

	flag.Parse()

	os.Chdir("/")
	syscall.Umask(0000)

	ConfigLoad()
	LoadScores()
	ProcessingLoop()
}
