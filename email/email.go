package email

import (
	"fmt"
	"net/smtp"
	"os"
)

// SendVerificationEmail sends an account-activation email to the invited user.
// Required env vars: SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_FROM, APP_URL.
// Uses STARTTLS (port 587). Port 465 (implicit TLS) is not supported.
func SendVerificationEmail(to, username, token string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	appURL := os.Getenv("APP_URL")

	verifyURL := fmt.Sprintf("%s/verify?token=%s", appURL, token)

	body := fmt.Sprintf(
		`Hi %s,

An admin has created an account for you.

Click the link below to set your password (expires in 24 hours):

%s

If you did not expect this email, you can ignore it.
`,
		username, verifyURL,
	)

	msg := []byte(
		"From: " + from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: You've been invited to Ledger\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" +
			body,
	)

	auth := smtp.PlainAuth("", user, pass, host)
	return smtp.SendMail(host+":"+port, auth, from, []string{to}, msg)
}
