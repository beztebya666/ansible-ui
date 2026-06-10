// Package notify delivers run-completion alerts to external channels
// (Telegram, Slack, generic webhook, email/SMTP).
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// The HTTP client for chat/webhook channels. Held in an atomic pointer so an
// outbound proxy (egress control for restricted networks) can be swapped in at
// runtime via SetProxy without racing in-flight dispatches.
var clientPtr atomic.Pointer[http.Client]

func init() { clientPtr.Store(&http.Client{Timeout: 10 * time.Second}) }

func httpClient() *http.Client { return clientPtr.Load() }

// SetProxy routes outbound notification HTTP through proxyURL (http/https/socks5).
// Empty clears any proxy (direct egress). Invalid URLs return an error and leave
// the current client unchanged.
func SetProxy(proxyURL string) error {
	proxyURL = strings.TrimSpace(proxyURL)
	c := &http.Client{Timeout: 10 * time.Second}
	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return err
		}
		c.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
	}
	clientPtr.Store(c)
	return nil
}

// Dispatch sends a message to a channel. title is a short headline, text the
// body, link an optional URL to the run, and vars the structured run fields
// (merged into the generic webhook payload for automation consumers).
func Dispatch(ctx context.Context, ch *model.NotificationChannel, title, text, link string, vars map[string]string) error {
	switch ch.Type {
	case model.NotifyTelegram:
		return sendTelegram(ctx, ch, title, text, link)
	case model.NotifySlack:
		return sendSlack(ctx, ch, title, text, link)
	case model.NotifyWebhook:
		return sendWebhook(ctx, ch, title, text, link, vars)
	case model.NotifyEmail:
		return sendEmail(ch, title, text, link)
	case model.NotifyDiscord:
		return sendDiscord(ctx, ch, title, text, link)
	case model.NotifyTeams:
		return sendTeams(ctx, ch, title, text, link)
	case model.NotifyGotify:
		return sendGotify(ctx, ch, title, text, link)
	case model.NotifyRocketchat, model.NotifyGoogleChat:
		return sendTextHook(ctx, ch, title, text, link) // both take {"text": …}
	case model.NotifyNtfy:
		return sendNtfy(ctx, ch, title, text, link)
	case model.NotifyPushover:
		return sendPushover(ctx, ch, title, text, link)
	case model.NotifyDingtalk:
		return sendDingtalk(ctx, ch, title, text, link)
	case model.NotifyPagerDuty:
		return sendPagerDuty(ctx, ch, title, text, vars)
	case model.NotifyOpsgenie:
		return sendOpsgenie(ctx, ch, title, text, vars)
	default:
		return fmt.Errorf("unknown channel type %q", ch.Type)
	}
}

// body joins title + text + link into one plain message (used by most chat hooks).
func bodyOf(title, text, link string) string {
	b := title + "\n" + text
	if link != "" {
		b += "\n" + link
	}
	return b
}

// sendDiscord posts to a Discord incoming webhook ({"content": …}).
func sendDiscord(ctx context.Context, ch *model.NotificationChannel, title, text, link string) error {
	hook := ch.Config["url"]
	if hook == "" {
		return fmt.Errorf("discord channel needs a webhook url")
	}
	return postJSON(ctx, hook, map[string]any{"content": bodyOf(title, text, link)})
}

// sendTeams posts a MessageCard to a Microsoft Teams incoming webhook.
func sendTeams(ctx context.Context, ch *model.NotificationChannel, title, text, link string) error {
	hook := ch.Config["url"]
	if hook == "" {
		return fmt.Errorf("teams channel needs a webhook url")
	}
	body := text
	if link != "" {
		body += "\n\n" + link
	}
	return postJSON(ctx, hook, map[string]any{
		"@type": "MessageCard", "@context": "http://schema.org/extensions",
		"summary": title, "title": title, "text": body,
	})
}

// sendGotify posts to a Gotify server ({server}/message, X-Gotify-Key header).
func sendGotify(ctx context.Context, ch *model.NotificationChannel, title, text, link string) error {
	server := strings.TrimRight(strings.TrimSpace(ch.Config["url"]), "/")
	token := strings.TrimSpace(ch.Config["token"])
	if server == "" || token == "" {
		return fmt.Errorf("gotify channel needs url and token")
	}
	msg := text
	if link != "" {
		msg += "\n" + link
	}
	b, _ := json.Marshal(map[string]any{"title": title, "message": msg, "priority": 5})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/message", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gotify-Key", token)
	return do(req)
}

// sendTextHook posts {"text": …} — Rocket.Chat + Google Chat incoming webhooks.
func sendTextHook(ctx context.Context, ch *model.NotificationChannel, title, text, link string) error {
	hook := ch.Config["url"]
	if hook == "" {
		return fmt.Errorf("channel needs a webhook url")
	}
	return postJSON(ctx, hook, map[string]any{"text": bodyOf(title, text, link)})
}

// sendNtfy publishes to an ntfy topic (POST the body, Title header).
func sendNtfy(ctx context.Context, ch *model.NotificationChannel, title, text, link string) error {
	topic := strings.TrimSpace(ch.Config["url"])
	if topic == "" {
		return fmt.Errorf("ntfy channel needs a topic url")
	}
	msg := text
	if link != "" {
		msg += "\n" + link
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, topic, strings.NewReader(msg))
	if err != nil {
		return err
	}
	req.Header.Set("Title", title)
	if tok := strings.TrimSpace(ch.Config["token"]); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return do(req)
}

// sendPushover posts to the Pushover messages API (token + user).
func sendPushover(ctx context.Context, ch *model.NotificationChannel, title, text, link string) error {
	token := strings.TrimSpace(ch.Config["token"])
	user := strings.TrimSpace(ch.Config["user"])
	if token == "" || user == "" {
		return fmt.Errorf("pushover channel needs token and user")
	}
	form := url.Values{"token": {token}, "user": {user}, "title": {title}, "message": {text}}
	if link != "" {
		form.Set("url", link)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.pushover.net/1/messages.json", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(req)
}

// sendPagerDuty triggers a PagerDuty Events API v2 alert. routingKey is the
// integration key; the endpoint defaults to PagerDuty's cloud but is overridable
// (on-prem / testing). Severity is derived from the run status.
func sendPagerDuty(ctx context.Context, ch *model.NotificationChannel, title, text string, vars map[string]string) error {
	key := strings.TrimSpace(ch.Config["routingKey"])
	if key == "" {
		return fmt.Errorf("pagerduty channel needs a routingKey")
	}
	endpoint := strings.TrimSpace(ch.Config["endpoint"])
	if endpoint == "" {
		endpoint = "https://events.pagerduty.com/v2/enqueue"
	}
	severity := "info"
	switch vars["status"] {
	case "failed", "canceled":
		severity = "error"
	}
	payload := map[string]any{
		"routing_key":  key,
		"event_action": "trigger",
		"payload": map[string]any{
			"summary":  title + " — " + text,
			"severity": severity,
			"source":   "ansible-ui",
		},
	}
	if id := vars["id"]; id != "" {
		payload["dedup_key"] = id
	}
	return postJSON(ctx, endpoint, payload)
}

// sendOpsgenie creates an Opsgenie alert (Alert API v2). apiKey authenticates as
// "GenieKey <key>"; the endpoint defaults to Opsgenie's cloud but is overridable
// (EU region / on-prem / testing). Priority is derived from the run status.
func sendOpsgenie(ctx context.Context, ch *model.NotificationChannel, title, text string, vars map[string]string) error {
	key := strings.TrimSpace(ch.Config["apiKey"])
	if key == "" {
		return fmt.Errorf("opsgenie channel needs an apiKey")
	}
	endpoint := strings.TrimSpace(ch.Config["endpoint"])
	if endpoint == "" {
		endpoint = "https://api.opsgenie.com/v2/alerts"
	}
	priority := "P3"
	switch vars["status"] {
	case "failed", "canceled":
		priority = "P1"
	}
	payload := map[string]any{
		"message":     title,
		"description": text,
		"priority":    priority,
		"source":      "ansible-ui",
	}
	if id := vars["id"]; id != "" {
		payload["alias"] = id // de-dupe key
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "GenieKey "+key)
	return do(req)
}

// sendDingtalk posts a text message to a DingTalk robot webhook.
func sendDingtalk(ctx context.Context, ch *model.NotificationChannel, title, text, link string) error {
	hook := ch.Config["url"]
	if hook == "" {
		return fmt.Errorf("dingtalk channel needs a webhook url")
	}
	return postJSON(ctx, hook, map[string]any{
		"msgtype": "text", "text": map[string]string{"content": bodyOf(title, text, link)},
	})
}

// sendEmail delivers the alert over SMTP. Config: host, port (default 587),
// from, to (comma-separated), and optional username/password (PlainAuth).
// net/smtp upgrades to STARTTLS automatically when the server advertises it.
func sendEmail(ch *model.NotificationChannel, title, text, link string) error {
	host := strings.TrimSpace(ch.Config["host"])
	from := strings.TrimSpace(ch.Config["from"])
	to := strings.TrimSpace(ch.Config["to"])
	if host == "" || from == "" || to == "" {
		return fmt.Errorf("email channel needs host, from and to")
	}
	port := strings.TrimSpace(ch.Config["port"])
	if port == "" {
		port = "587"
	}
	recipients := make([]string, 0)
	for _, r := range strings.Split(to, ",") {
		if r = strings.TrimSpace(r); r != "" {
			recipients = append(recipients, r)
		}
	}
	body := text
	if link != "" {
		body += "\r\n\r\n" + link
	}
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(recipients, ", ") + "\r\n")
	b.WriteString("Subject: " + title + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(body + "\r\n")

	var auth smtp.Auth
	if user := strings.TrimSpace(ch.Config["username"]); user != "" {
		auth = smtp.PlainAuth("", user, ch.Config["password"], host)
	}
	return smtp.SendMail(net.JoinHostPort(host, port), auth, from, recipients, []byte(b.String()))
}

func sendTelegram(ctx context.Context, ch *model.NotificationChannel, title, text, link string) error {
	token := ch.Config["botToken"]
	chatID := ch.Config["chatId"]
	if token == "" || chatID == "" {
		return fmt.Errorf("telegram channel needs botToken and chatId")
	}
	msg := title + "\n" + text
	if link != "" {
		msg += "\n" + link
	}
	form := url.Values{"chat_id": {chatID}, "text": {msg}, "disable_web_page_preview": {"true"}}
	if tid := strings.TrimSpace(ch.Config["threadId"]); tid != "" {
		form.Set("message_thread_id", tid) // forum topic / thread
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.telegram.org/bot"+token+"/sendMessage", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(req)
}

func sendSlack(ctx context.Context, ch *model.NotificationChannel, title, text, link string) error {
	hook := ch.Config["url"]
	if hook == "" {
		return fmt.Errorf("slack channel needs a webhook url")
	}
	body := title + "\n" + text
	if link != "" {
		body += "\n<" + link + "|open run>"
	}
	return postJSON(ctx, hook, map[string]any{"text": body})
}

func sendWebhook(ctx context.Context, ch *model.NotificationChannel, title, text, link string, vars map[string]string) error {
	hook := ch.Config["url"]
	if hook == "" {
		return fmt.Errorf("webhook channel needs a url")
	}
	// Backward-compatible: keep title/text/link, plus the structured run fields
	// (run/status/project/app/playbook/commit/version/actor/exitCode/id).
	payload := map[string]any{"title": title, "text": text, "link": link}
	for k, v := range vars {
		payload[k] = v
	}
	return postJSON(ctx, hook, payload)
}

func postJSON(ctx context.Context, urlStr string, payload any) error {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return do(req)
}

func do(req *http.Request) error {
	resp, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify: HTTP %d", resp.StatusCode)
	}
	return nil
}
