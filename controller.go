package gobearmon

import "bufio"
import "database/sql"
import "encoding/json"
import "errors"
import "log"
import "math/rand"
import "net"
import "strings"
import "sync"
import "time"

type Controller struct {
	Addr string
	Databases []*sql.DB
	Confirmations int
	mu sync.Mutex
	checks map[CheckId]*Check
}

func (this *Controller) Start() {
	this.checks = make(map[CheckId]*Check)

	ln, err := net.Listen("tcp", this.Addr)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("controller: error while accepting connection: %s", err.Error())
				continue
			}
			log.Printf("controller: new connection from %s", conn.RemoteAddr().String())
			go this.handle(conn)
		}
	}()

	go func() {
		for {
			this.reload()
			time.Sleep(time.Minute)
		}
	}()
}

func (this *Controller) GetCheck(checkId CheckId) *Check {
	this.mu.Lock()
	defer this.mu.Unlock()
	return this.checks[checkId]
}

func (this *Controller) randomDB() *sql.DB {
	return this.Databases[rand.Intn(len(this.Databases))]
}

func (this *Controller) handle(conn net.Conn) {
	defer conn.Close()
	in := bufio.NewReader(conn)

	password, err := in.ReadString('\n')
	if err != nil || len(password) == 0 || password[:len(password) - 1] != cfg.Default.Password {
		log.Printf("controller: terminating connection from %s due to incorrect password", conn.RemoteAddr().String())
		return
	}

	for {
		line, err := in.ReadString('\n')
		if err != nil {
			log.Printf("controller: worker at %s disconnected: %s", conn.RemoteAddr().String(), err.Error())
			break
		}

		request := MakeControllerRequest()
		err = json.Unmarshal([]byte(strings.TrimSpace(line)), request)
		if err != nil {
			log.Printf("controller: invalid request from %s: %s", conn.RemoteAddr().String(), err.Error())
			break
		}
		response := this.request(conn.RemoteAddr().String(), request)
		bytes, err := json.Marshal(response)
		if err != nil {
			panic(err)
		}
		conn.Write([]byte(string(bytes) + "\n"))
	}
}

func (this *Controller) request(requestor string, request *ControllerRequest) *ControllerResponse {
	this.mu.Lock()
	defer this.mu.Unlock()

	for checkId, checkResult := range request.Results {
		check := this.checks[checkId]
		if check == nil {
			// probably got deleted from database during checking
			continue
		} else if requestor != check.Lock {
			continue
		}

		check.Lock = ""
		check.LastTime = time.Now()
		check.LastWorker = requestor

		if checkResult.Status != StatusOnline && checkResult.Status != StatusOffline {
			continue
		}

		if checkResult.Status != check.Status {
			check.TurnSet[requestor] = true
			if len(check.TurnSet) >= this.Confirmations {
				for id := range check.TurnSet {
					delete(check.TurnSet, id)
				}
				check.TurnCount++
				debugPrintf("check [%s]: turn count incremented to %d/%d", check.Name, check.TurnCount, check.Delay + 1)
				if check.TurnCount > check.Delay {
					check.Status = checkResult.Status
					check.LastStatusChange = time.Now()
					log.Printf("status of check %s changed to %s", check.Name, check.Status)
					go this.reportAndUpdate(check, checkResult)
				}
			}
		} else {
			check.TurnCount = 0
			for id := range check.TurnSet {
				delete(check.TurnSet, id)
			}
		}
	}

	var response ControllerResponse
	for checkId, check := range this.checks {
		if len(response.Checks) >= request.Count {
			break
		} else if check.Lock != "" {
			continue
		}

		assign := false

		if len(check.TurnSet) > 0 {
			if !check.TurnSet[requestor] {
				assign = true
			}
		} else if time.Now().After(check.LastTime.Add(time.Duration(check.Interval) * time.Second)) && check.LastWorker != requestor {
			assign = true
		}

		if assign {
			check.Lock = requestor
			check.LockTime = time.Now()
			response.Checks = append(response.Checks, checkId)
		}
	}

	return &response
}

func (this *Controller) reportAndUpdate(check *Check, result *CheckResult) {
	// attempt reporting
	// if we succeed, then update the database
	// if we fail, then reset the check status
	success := retry(func() error {
		return this.report(check, result)
	}, 10)

	if success {
		retry(func() error {
			_, err := this.Databases[rand.Intn(len(this.Databases))].Exec("UPDATE checks SET status = ? WHERE id = ?", string(result.Status), check.Id)
			return err
		}, 10)
		retry(func() error {
			_, err := this.Databases[rand.Intn(len(this.Databases))].Exec("INSERT INTO check_events (check_id, type) VALUES (?, ?)", check.Id, string(result.Status))
			return err
		}, 10)
	} else {
		this.mu.Lock()
		if result.Status == StatusOnline {
			check.Status = StatusOffline
		} else if result.Status == StatusOffline {
			check.Status = StatusOnline
		}
		this.mu.Unlock()
	}
}

func (this *Controller) report(check *Check, result *CheckResult) error {
	rows, err := this.randomDB().Query("SELECT contacts.type, contacts.data FROM contacts, alerts WHERE alerts.check_id = ? AND alerts.contact_id = contacts.id AND (alerts.type = 'both' OR alerts.type = ?)", check.Id, string(result.Status))
	if err != nil {
		return errors.New("database query failed")
	}
	var alerts []*Alert
	for rows.Next() {
		var alert Alert
		err := rows.Scan(&alert.Type, &alert.Data)
		if err != nil {
			return errors.New("database query failed")
		}
		alerts = append(alerts, &alert)
	}

	if len(alerts) == 0 {
		return nil
	}

	// we iterate over the contacts and alert each one
	// if at least one succeeds, we report success to caller to avoid duplicate alerting
	// we retry the ones that failed after some delay; only one retry is attempted
	atLeastOneSuccess := false
	var failedAlerts []*Alert
	for _, alert := range alerts {
		err := DoAlert(alert, check, result, this.randomDB())
		if err == nil {
			atLeastOneSuccess = true
		} else {
			failedAlerts = append(failedAlerts, alert)
			debugPrintf("failed to alert %s/%s: %s (trying again later)", alert.Type, alert.Data, err.Error())
		}
	}

	if !atLeastOneSuccess {
		return errors.New("all alerts failed")
	}

	if len(failedAlerts) > 0 {
		go func() {
			time.Sleep(30 * time.Second)
			for _, alert := range failedAlerts {
				err := DoAlert(alert, check, result, this.randomDB())
				log.Printf("permanently failed to alert %s/%s: %s", alert.Type, alert.Data, err.Error())
			}
		}()
	}

	return nil
}

func (this *Controller) reload() {
	db := this.randomDB()
	rows, err := db.Query("SELECT id, name, type, data, check_interval, delay, status FROM checks")
	if err != nil {
		log.Printf("controller: reload error on query: %s", err.Error())
		return
	}

	var dbChecks []*Check
	existCheckIds := make(map[CheckId]bool)

	for rows.Next() {
		check := MakeCheck()
		var statusString string
		err := rows.Scan(&check.Id, &check.Name, &check.Type, &check.Data, &check.Interval, &check.Delay, &statusString)
		check.SetStatusFromString(statusString)
		if err != nil {
			log.Printf("controller: reload error on scan: %s", err.Error())
			return
		}
		existCheckIds[check.Id] = true
		dbChecks = append(dbChecks, check)
	}

	this.mu.Lock()
	defer this.mu.Unlock()

	// insert/update
	for _, dbCheck := range dbChecks {
		check := this.checks[dbCheck.Id]
		if check == nil {
			this.checks[dbCheck.Id] = dbCheck
		} else {
			check.Name = dbCheck.Name
			check.Type = dbCheck.Type
			check.Data = dbCheck.Data
			check.Interval = dbCheck.Interval
			check.Delay = dbCheck.Delay

			// only copy status if we didn't update the status recently
			if time.Now().After(check.LastStatusChange.Add(10 * time.Minute)) {
				check.Status = dbCheck.Status
			}
		}
	}

	// delete; also hijack to remove locks
	for checkId, check := range this.checks {
		if !existCheckIds[checkId] {
			delete(this.checks, checkId)
		} else if check.Lock != "" && time.Now().After(check.LockTime.Add(2 * time.Minute)) {
			check.Lock = ""
		}
	}
}
