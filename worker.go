package gobearmon

import "bufio"
import "encoding/json"
import "log"
import "net"
import "strings"
import "sync"
import "time"

type Worker struct {
	ViewAddr string
	Controller *Controller
	NumThreads int
	mu sync.Mutex
	activeController string
	availableWorkers map[int]chan CheckId
	pendingResults map[CheckId]*CheckResult
}

func (this *Worker) Start() {
	this.availableWorkers = make(map[int]chan CheckId)
	this.pendingResults = make(map[CheckId]*CheckResult)

	go this.updateController()
	go this.updateView()

	for i := 0; i < this.NumThreads; i++ {
		go this.task(i)
	}
}

func (this *Worker) updateController() {
	var currentController string
	var conn net.Conn
	var in *bufio.Reader

	for {
		if conn == nil {
			currentController = this.getActiveController()
			if currentController != "" {
				log.Printf("worker: connecting to controller at %s", currentController)
				var err error
				conn, err = net.DialTimeout("tcp", currentController, 10 * time.Second)
				if err != nil {
					log.Printf("worker: connect error: %s", err.Error())
					time.Sleep(10 * time.Second)
					continue
				}
				conn.Write([]byte(cfg.Default.Password + "\n"))
				in = bufio.NewReader(conn)
			} else {
				time.Sleep(5 * time.Second)
				continue
			}
		} else {
			activeController := this.getActiveController()
			if currentController != activeController {
				log.Printf("worker: controller changed from %s to %s, disconnecting", currentController, activeController)
				conn.Close()
				conn = nil
				continue
			}
		}

		// do controller request
		request := MakeControllerRequest()
		this.mu.Lock()
		request.Count = len(this.availableWorkers)
		for checkId, checkResult := range this.pendingResults {
			request.Results[checkId] = checkResult
		}
		this.mu.Unlock()
		requestBytes, err := json.Marshal(request)
		if err != nil {
			panic(err)
		}
		conn.Write([]byte(string(requestBytes) + "\n"))

		// decode response
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := in.ReadString('\n')
		if err != nil {
			log.Printf("worker: controller disconnected: %s", err.Error())
			conn.Close()
			conn = nil
			time.Sleep(10 * time.Second)
			continue
		}
		var response ControllerResponse
		err = json.Unmarshal([]byte(strings.TrimSpace(line)), &response)
		if err != nil {
			log.Printf("worker: controller json error: %s", err.Error())
			conn.Close()
			conn = nil
			time.Sleep(10 * time.Second)
			continue
		}

		// process response
		this.mu.Lock()
		for checkId := range request.Results {
			delete(this.pendingResults, checkId)
		}

		idx := 0
		for workerId, ch := range this.availableWorkers {
			if idx >= len(response.Checks) {
				break
			}
			ch <- response.Checks[idx]
			delete(this.availableWorkers, workerId)
			idx++
		}
		if idx != len(response.Checks) {
			log.Println("worker: warning: got more checks than able to distribute!")
		}
		this.mu.Unlock()

		time.Sleep(2 * time.Second)
	}
}

func (this *Worker) getActiveController() string {
	this.mu.Lock()
	defer this.mu.Unlock()
	return this.activeController
}

func (this *Worker) setActiveController(controller string) {
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.activeController != controller {
		log.Printf("worker: updating controller from %s to %s", this.activeController, controller)
		this.activeController = controller
	}
}

func (this *Worker) updateView() {
	var conn net.Conn
	var in *bufio.Reader

	for {
		if conn == nil {
			log.Printf("worker: connecting to viewserver at %s", this.ViewAddr)
			var err error
			conn, err = net.DialTimeout("tcp", this.ViewAddr, 10 * time.Second)
			if err != nil {
				log.Printf("worker: viewserver connect error: %s", err.Error())
				time.Sleep(10 * time.Second)
				continue
			}
			conn.Write([]byte(cfg.Default.Password + "\n"))
			in = bufio.NewReader(conn)
		}

		conn.Write([]byte("request\n"))
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := in.ReadString('\n')
		if err != nil {
			log.Printf("worker: viewserver disconnected: %s", err.Error())
			conn.Close()
			conn = nil
			time.Sleep(10 * time.Second)
			continue
		}
		this.setActiveController(strings.TrimSpace(line))
		time.Sleep(10 * time.Second)
	}
}

func (this *Worker) task(workerId int) {
	ch := make(chan CheckId)

	for {
		this.mu.Lock()
		this.availableWorkers[workerId] = ch
		this.mu.Unlock()

		checkId := <- ch
		check := this.Controller.GetCheck(checkId)
		var result *CheckResult
		if check == nil {
			result = &CheckResult{
				Status: StatusFail,
				Message: "check does not exist",
			}
			log.Printf("assigned check id=%d, but check not found in local store", checkId)
		} else {
			result = DoCheck(check)
		}

		this.mu.Lock()
		this.pendingResults[checkId] = result
		this.mu.Unlock()
	}
}
