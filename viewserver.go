package gobearmon

import "bufio"
import "encoding/json"
import "log"
import "net"
import "strings"
import "sync"
import "time"

type ViewServer struct {
	Addr string
	Controllers []string
	pingStatus map[string]int
	activeController string
	mu sync.Mutex
}

func (this *ViewServer) Start() {
	this.pingStatus = make(map[string]int)

	for _, controller := range this.Controllers {
		this.pingStatus[controller] = 0
		go this.ping(controller)
	}

	ln, err := net.Listen("tcp", this.Addr)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("viewserver: error while accepting connection: %s", err.Error())
				continue
			}
			log.Printf("viewserver: new connection from %s", conn.RemoteAddr().String())
			go this.handle(conn)
		}
	}()
}

func (this *ViewServer) getActiveController() string {
	this.mu.Lock()
	defer this.mu.Unlock()
	return this.activeController
}

func (this *ViewServer) updatePing(controller string, result bool) {
	this.mu.Lock()
	defer this.mu.Unlock()

	if result {
		this.pingStatus[controller]++
		if this.activeController == "" {
			log.Printf("viewserver: initializing controller to %s", controller)
			this.activeController = controller
		}
	} else {
		log.Printf("viewserver: marking controller %s as down", controller)
		this.pingStatus[controller] = 0

		if controller == this.activeController {
			this.activeController = ""
			for testController, uptime := range this.pingStatus {
				if uptime > 0 && (this.activeController == "" || uptime > this.pingStatus[this.activeController]) {
					this.activeController = testController
				}
			}
			if this.activeController == "" {
				log.Printf("viewserver: warning: controller %s failed, but no one found to replace", controller)
			} else {
				log.Printf("viewserver: failover from %s to %s", controller, this.activeController)
			}
		}
	}
}

func (this *ViewServer) handle(conn net.Conn) {
	defer conn.Close()
	in := bufio.NewReader(conn)

	for {
		line, err := in.ReadString('\n')
		if err != nil {
			log.Printf("viewserver: client at %s disconnected: %s", conn.RemoteAddr().String(), err.Error())
			break
		}
		line = strings.TrimSpace(line)
		if line != "request" {
			log.Printf("viewserver: received invalid request [%s] from %s", line, conn.RemoteAddr().String())
			break
		}

		conn.Write([]byte(this.getActiveController() + "\n"))
	}
}

func (this *ViewServer) ping(controller string) {
	var conn net.Conn
	var in *bufio.Reader

	for {
		if conn == nil {
			var err error
			conn, err = net.DialTimeout("tcp", controller, 10 * time.Second)
			if err != nil {
				this.updatePing(controller, false)
				time.Sleep(30 * time.Second)
				continue
			}
			in = bufio.NewReader(conn)
		}

		// send empty request and expect empty but valid response
		requestBytes, err := json.Marshal(MakeControllerRequest())
		if err != nil {
			panic(err)
		}
		conn.Write([]byte(string(requestBytes) + "\n"))
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := in.ReadString('\n')
		if err != nil {
			this.updatePing(controller, false)
			conn.Close()
			conn = nil
			time.Sleep(30 * time.Second)
			continue
		}
		var response ControllerResponse
		err = json.Unmarshal([]byte(strings.TrimSpace(line)), &response)
		if err != nil {
			this.updatePing(controller, false)
			conn.Close()
			conn = nil
			time.Sleep(30 * time.Second)
			continue
		}

		this.updatePing(controller, true)

		time.Sleep(5 * time.Second)
	}
}
