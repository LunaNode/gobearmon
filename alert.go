package gobearmon

import "github.com/sfreiberg/gotwilio"

import "errors"
import "fmt"
import "net/http"
import "net/url"
import "database/sql"
import "strconv"

type AlertFunc func(string, *Check, *CheckResult, *sql.DB) error
var alertFuncs map[string]AlertFunc

func DoAlert(alert *Alert, check *Check, result *CheckResult, db *sql.DB) error {
	if alertFuncs == nil {
		alertInit()
	}
	f := alertFuncs[alert.Type]
	if f == nil {
		return errors.New(fmt.Sprintf("alert type %s does not exist", alert.Type))
	} else {
		debugPrintf("executing alert %s/%s for check [%s]", alert.Type, alert.Data, check.Name)
		return f(alert.Data, check, result, db)
	}
}

func alertInit() {
	alertFuncs = make(map[string]AlertFunc)

	alertFuncs["email"] = func(data string, check *Check, result *CheckResult, db *sql.DB) error {
		subject := fmt.Sprintf("Check %s: %s", result.Status, check.Name)

		var body string
		if result.Status == StatusOnline {
			body = fmt.Sprintf("Check [%s] is now online.", check.Name)
		} else {
			body = fmt.Sprintf("Check [%s] is now %s: %s", check.Name, result.Status, result.Message)
		}
		body += fmt.Sprintf("\n\nID: %d\nName: %s\nType: %s\nData: %s\n\ngobearmon", check.Id, check.Name, check.Type, check.Data)
		return mail(subject, body, data)
	}

	alertFuncs["http"] = func(data string, check *Check, result *CheckResult, db *sql.DB) error {
		resp, err := http.PostForm(data, url.Values{
			"check_id": {strconv.Itoa(int(check.Id))},
			"name": {check.Name},
			"type": {check.Type},
			"data": {check.Data},
			"status": {string(result.Status)},
			"message": {result.Message},
		})
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}

	alertFuncs["sms"] = func(data string, check *Check, result *CheckResult, db *sql.DB) error {
		var message string
		if result.Status == StatusOnline {
			message = fmt.Sprintf("Check [%s] is now online.", check.Name)
		} else {
			message = fmt.Sprintf("Check [%s] is now %s: %s", check.Name, result.Status, result.Message)
		}
		twilio := gotwilio.NewTwilioClient(cfg.Twilio.AccountSid, cfg.Twilio.AuthToken)
		resp, exception, err := twilio.SendSMS(cfg.Twilio.From, data, message, "", "")
		if err != nil {
			return err
		} else if exception != nil {
			return errors.New(fmt.Sprintf("error(%d/%d): %s (%s)", exception.Status, exception.Code, exception.Message, exception.MoreInfo))
		}
		db.Exec("INSERT INTO charges (check_id, type, data) VALUES (?, ?, ?)", check.Id, "sms", resp.Sid)
		return nil
	}

	alertFuncs["voice"] = func(data string, check *Check, result *CheckResult, db *sql.DB) error {
		message := fmt.Sprintf("This is a monitoring-related call from go bear mon . The check . %s . has been recorded . %s . Reason is . %s", check.Name, result.Status, result.Message)
		twilio := gotwilio.NewTwilioClient(cfg.Twilio.AccountSid, cfg.Twilio.AuthToken)
		params := gotwilio.NewCallbackParameters("http://twimlets.com/message?Message=" + url.QueryEscape(message))
		resp, exception, err := twilio.CallWithUrlCallbacks(cfg.Twilio.From, data, params)
		if err != nil {
			return err
		} else if exception != nil {
			return errors.New(fmt.Sprintf("error(%d/%d): %s (%s)", exception.Status, exception.Code, exception.Message, exception.MoreInfo))
		}
		db.Exec("INSERT INTO charges (check_id, type, data) VALUES (?, ?, ?)", check.Id, "voice", resp.Sid)
		return nil
	}
}
