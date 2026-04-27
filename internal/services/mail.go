package services

import (
	"fmt"

	"github.com/alemancenter/fiber-api/internal/config"
	mail "github.com/wneessen/go-mail"
)

// MailService handles email sending
type MailService struct {
	cfg config.MailConfig
}

// NewMailService creates a new MailService
func NewMailService() *MailService {
	return &MailService{cfg: config.Get().Mail}
}

// Send sends a plain text or HTML email
func (m *MailService) Send(to, subject, body string, isHTML bool) error {
	msg := mail.NewMsg()
	if err := msg.From(fmt.Sprintf("%s <%s>", m.cfg.FromName, m.cfg.FromAddress)); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("invalid to address: %w", err)
	}
	msg.Subject(subject)

	if isHTML {
		msg.SetBodyHTMLTemplate(nil, nil)
		msg.SetBodyString(mail.TypeTextHTML, body)
	} else {
		msg.SetBodyString(mail.TypeTextPlain, body)
	}

	port := m.cfg.Port
	tlsMode := mail.TLSMandatory
	if m.cfg.Encryption == "ssl" {
		tlsMode = mail.TLSMandatory
	} else if m.cfg.Encryption == "tls" {
		tlsMode = mail.TLSMandatory
	} else {
		tlsMode = mail.TLSOpportunistic
	}

	client, err := mail.NewClient(m.cfg.Host,
		mail.WithPort(port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(m.cfg.Username),
		mail.WithPassword(m.cfg.Password),
		mail.WithTLSPolicy(tlsMode),
	)
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	return client.DialAndSend(msg)
}

// SendVerificationEmail sends an email verification link
func (m *MailService) SendVerificationEmail(to, name, verificationURL string) error {
	subject := "تحقق من بريدك الإلكتروني - Alemancenter"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html dir="rtl" lang="ar">
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; direction: rtl; text-align: right; background: #f5f5f5; padding: 20px;">
  <div style="max-width: 600px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px;">
    <h2 style="color: #333;">مرحباً %s،</h2>
    <p>شكراً لتسجيلك في Alemancenter. يرجى النقر على الرابط أدناه للتحقق من بريدك الإلكتروني:</p>
    <div style="text-align: center; margin: 30px 0;">
      <a href="%s" style="background: #0d6efd; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">
        تحقق من البريد الإلكتروني
      </a>
    </div>
    <p style="color: #666; font-size: 14px;">ينتهي هذا الرابط خلال 60 دقيقة. إذا لم تقم بإنشاء حساب، تجاهل هذا البريد.</p>
  </div>
</body>
</html>`, name, verificationURL)

	return m.Send(to, subject, body, true)
}

// SendPasswordResetEmail sends a password reset link
func (m *MailService) SendPasswordResetEmail(to, name, resetURL string) error {
	subject := "إعادة تعيين كلمة المرور - Alemancenter"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html dir="rtl" lang="ar">
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; direction: rtl; text-align: right; background: #f5f5f5; padding: 20px;">
  <div style="max-width: 600px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px;">
    <h2 style="color: #333;">مرحباً %s،</h2>
    <p>تلقينا طلباً لإعادة تعيين كلمة المرور الخاصة بك. انقر على الرابط أدناه للمتابعة:</p>
    <div style="text-align: center; margin: 30px 0;">
      <a href="%s" style="background: #dc3545; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">
        إعادة تعيين كلمة المرور
      </a>
    </div>
    <p style="color: #666; font-size: 14px;">ينتهي هذا الرابط خلال 60 دقيقة. إذا لم تطلب إعادة التعيين، تجاهل هذا البريد.</p>
  </div>
</body>
</html>`, name, resetURL)

	return m.Send(to, subject, body, true)
}

// TestSMTP tests the SMTP connection
func (m *MailService) TestSMTP() error {
	return m.Send(m.cfg.FromAddress, "SMTP Test - Alemancenter", "SMTP connection test successful.", false)
}
