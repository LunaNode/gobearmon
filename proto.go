package gobearmon

import "encoding/json"
import "errors"
import "strconv"
import "time"

type CheckId int
type CheckStatus string
type CheckResults map[CheckId]*CheckResult

const (
	StatusOffline CheckStatus = "offline"
	StatusOnline = "online"
	StatusFail = "fail"
)

type CheckResult struct {
	Status CheckStatus `json:"status"`
	Message string `json:"message"`
}

type ControllerRequest struct {
	Results CheckResults `json:"results"`
	Count int `json:"count"`
}

func MakeControllerRequest() *ControllerRequest {
	return &ControllerRequest{
		Results: make(CheckResults),
	}
}

type ControllerResponse struct {
	Checks []CheckId `json:"checks"`
}

type Check struct {
	Id CheckId
	Name string
	Type string
	Data string
	Interval int
	Delay int
	Status CheckStatus

	Lock string
	LockTime time.Time
	LastWorker string
	LastTime time.Time
	TurnSet map[string]bool
	TurnCount int
	LastStatusChange time.Time
}

func (check *Check) SetStatusFromString(status string) {
	if status == string(StatusOffline) {
		check.Status = StatusOffline
	} else if status == string(StatusOnline) {
		check.Status = StatusOnline
	} else {
		panic(errors.New("invalid check status to set: " + status))
	}
}

func MakeCheck() *Check {
	return &Check{
		TurnSet: make(map[string]bool),
	}
}

type Alert struct {
	Type string
	Data string
}

func (results CheckResults) MarshalJSON() ([]byte, error) {
	stringMap := make(map[string]*CheckResult)
	for k, v := range results {
		stringMap[strconv.Itoa(int(k))] = v
	}
	return json.Marshal(stringMap)
}

func (results CheckResults) UnmarshalJSON(bytes []byte) error {
	stringMap := make(map[string]*CheckResult)
	err := json.Unmarshal(bytes, &stringMap)
	if err != nil {
		return err
	}

	for k, v := range stringMap {
		i, err := strconv.Atoi(k)
		if err != nil {
			return err
		}
		results[CheckId(i)] = v
	}
	return nil
}
