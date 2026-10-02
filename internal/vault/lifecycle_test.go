package vault

import (
	"crypto/cipher"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestUnlockDiscardsObsoleteResults(t *testing.T) {
	for _, password := range []string{"correct-password", "wrong-password"} {
		for _, next := range []string{"lock", "unlock"} {
			t.Run(password+"/"+next, func(t *testing.T) {
				service := testService(t)
				started, release := make(chan struct{}), make(chan struct{})
				resume := sync.OnceFunc(func() { close(release) })
				t.Cleanup(resume)
				result := make(chan error, 1)
				go func() {
					_, err := service.unlock(password, func(v EncryptedVault, p string) (DocumentManifest, cipher.AEAD, error) {
						close(started)
						<-release
						return decryptManifestWithPassword(v, p)
					})
					result <- err
				}()
				<-started
				if next == "lock" {
					service.Lock()
				} else if _, err := service.Unlock("correct-password"); err != nil {
					t.Fatal(err)
				}
				resume()
				if err := <-result; !errors.Is(err, ErrLocked) {
					t.Fatalf("obsolete unlock = %v, want ErrLocked", err)
				}
				_, err := service.GetTree()
				if (next == "lock" && !errors.Is(err, ErrLocked)) || (next == "unlock" && err != nil) {
					t.Fatalf("latest %s state changed: %v", next, err)
				}
				if service.unlockFailures != 0 {
					t.Fatal("obsolete failure affected current throttle")
				}
			})
		}
	}
}

func TestAIRequestOwnership(t *testing.T) {
	service, events := newAIStreamTestService(t, "http://unused.invalid")
	first, err := service.registerAIRequest(1, service.session)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.registerAIRequest(2, service.session)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { service.Lock() })
	if first.ctx.Err() == nil {
		t.Fatal("replacement did not cancel old request")
	}
	service.CancelAIChat(1)
	service.finishAIRequest(first)
	service.emitAI(aiEventChunk, first, "obsolete")
	if second.ctx.Err() != nil || service.aiRequest != second {
		t.Fatal("old cancellation/completion affected new request")
	}
	select {
	case event := <-events:
		t.Fatalf("obsolete event delivered: %+v", event)
	default:
	}
	service.CancelAIChat(2)
	if second.ctx.Err() == nil || service.aiRequest != nil {
		t.Fatal("matching cancellation did not clear request")
	}
}

func TestAIRegistrationRejectsObsoleteSession(t *testing.T) {
	service, _ := newAIStreamTestService(t, "http://unused.invalid")
	oldSession := service.session
	service.Lock()
	if _, err := service.registerAIRequest(1, oldSession); !errors.Is(err, ErrLocked) {
		t.Fatalf("registration after lock = %v", err)
	}
	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatal(err)
	}
	current, err := service.registerAIRequest(2, service.session)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Lock)
	if _, err := service.registerAIRequest(1, oldSession); !errors.Is(err, ErrLocked) {
		t.Fatalf("registration after re-unlock = %v", err)
	}
	if current.ctx.Err() != nil {
		t.Fatal("obsolete registration canceled current request")
	}
	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatal(err)
	}
	if current.ctx.Err() == nil {
		t.Fatal("re-unlock did not cancel the previous session request")
	}
}

func TestAIStreamRequiresCompletionMarker(t *testing.T) {
	for _, test := range []struct {
		name, tail, terminal string
	}{
		{"done", "data: [DONE]\n\n", aiEventDone},
		{"finish", "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n", aiEventDone},
		{"eof", "", aiEventError},
		{"malformed", "data: {broken\n\n", aiEventError},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"+test.tail)
			}))
			t.Cleanup(server.Close)
			service, events := newAIStreamTestService(t, server.URL)
			t.Cleanup(service.Lock)
			if err := service.AIChat(AIChatRequest{RequestID: 1, Question: "q"}); err != nil {
				t.Fatal(err)
			}
			if first := waitAIEvent(t, events); first.name != aiEventChunk || first.data != "partial" {
				t.Fatalf("first event = %+v", first)
			}
			terminal := waitAIEvent(t, events)
			if terminal.name != test.terminal {
				t.Fatalf("terminal = %+v, want %s", terminal, test.terminal)
			}
			if test.terminal == aiEventError && !strings.Contains(terminal.data, "不完整") {
				t.Fatalf("missing incomplete response explanation: %s", terminal.data)
			}
		})
	}
}
