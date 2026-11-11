package internal

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/freswa/dovecot-xaps-daemon/internal/database"
	"github.com/sideshow/apns2"
)

type notificationTransport func(*http.Request) (*http.Response, error)

func (f notificationTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestNotificationDeliveryPriority(t *testing.T) {
	for _, newMessage := range []bool{false, true} {
		name := "delayed"
		wantType, wantPriority := "background", "5"
		if newMessage {
			name = "new message supersedes queued notification"
			wantType, wantPriority = "alert", "10"
		}
		t.Run(name, func(t *testing.T) {
			requests := 0
			a := &Apns{
				DelayTime:   30,
				delayedApns: make(map[database.Registration]time.Time),
				client: &apns2.Client{
					Host: apns2.HostProduction,
					HTTPClient: &http.Client{Transport: notificationTransport(func(r *http.Request) (*http.Response, error) {
						requests++
						if got := r.Header.Get("apns-push-type"); got != wantType {
							t.Errorf("push type = %q, want %q", got, wantType)
						}
						if got := r.Header.Get("apns-priority"); got != wantPriority {
							t.Errorf("priority = %q, want %q", got, wantPriority)
						}
						return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
					})},
				},
			}
			reg := database.Registration{DeviceToken: "device", AccountId: "account"}
			a.SendNotification(reg, true)
			a.checkDelayed()
			if requests != 0 {
				t.Fatal("queued notification was sent before its delay elapsed")
			}
			if newMessage {
				a.SendNotification(reg, false)
			} else {
				a.delayedApns[reg] = time.Now().Add(-time.Minute)
			}
			a.checkDelayed()
			a.checkDelayed()
			if requests != 1 || len(a.delayedApns) != 0 {
				t.Fatalf("requests = %d, queued = %d; want one request and empty queue", requests, len(a.delayedApns))
			}
		})
	}
}
