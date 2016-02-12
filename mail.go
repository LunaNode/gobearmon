package gobearmon

import "gopkg.in/gomail.v1"

func mail(subject string, body string, to string) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", cfg.Smtp.From)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/plain", body)
	mailer := gomail.NewMailer(cfg.Smtp.Host, cfg.Smtp.Username, cfg.Smtp.Password, cfg.Smtp.Port)
	return mailer.Send(msg)
}

func mailAdmin(subject string, body string) error {
	return mail(subject, body, cfg.Smtp.Admin)
}
