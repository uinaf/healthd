package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/runner"
)

type State string

const (
	StateOK        State = "ok"
	StateWarn      State = "warn"
	StateCrit      State = "crit"
	StateRecovered State = "recovered"
)

type Event struct {
	CheckName string    `json:"check_name"`
	Group     string    `json:"group"`
	State     State     `json:"state"`
	Previous  State     `json:"previous"`
	Reason    string    `json:"reason"`
	ExitCode  int       `json:"exit_code"`
	Timestamp time.Time `json:"timestamp"`
}

type Notifier interface {
	Name() string
	Notify(context.Context, Event) error
}

type Tracker struct {
	cooldown time.Duration
	states   map[string]State
	lastSent map[string]time.Time
	now      func() time.Time
}

func NewTracker(cooldown time.Duration) *Tracker {
	return &Tracker{
		cooldown: cooldown,
		states:   map[string]State{},
		lastSent: map[string]time.Time{},
		now:      time.Now,
	}
}

func (t *Tracker) EventFor(result runner.CheckResult) (Event, bool) {
	current := stateForResult(result)
	previous, seen := t.states[result.Name]
	t.states[result.Name] = current

	if !seen {
		if current == StateOK {
			return Event{}, false
		}
		return t.emit(result, current, "", current)
	}

	if previous == current {
		return Event{}, false
	}

	eventState := current
	if current == StateOK && (previous == StateWarn || previous == StateCrit) {
		eventState = StateRecovered
	}

	return t.emit(result, current, previous, eventState)
}

func (t *Tracker) emit(result runner.CheckResult, current State, previous State, eventState State) (Event, bool) {
	now := t.now()
	if t.cooldown > 0 {
		if lastSent, ok := t.lastSent[result.Name]; ok {
			if now.Sub(lastSent) < t.cooldown {
				return Event{}, false
			}
		}
	}

	t.lastSent[result.Name] = now

	return Event{
		CheckName: result.Name,
		Group:     result.Group,
		State:     eventState,
		Previous:  previous,
		Reason:    result.Reason,
		ExitCode:  result.ExitCode,
		Timestamp: result.Timestamp,
	}, true
}

func ParseCooldown(raw string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse cooldown: %w", err)
	}
	if d < 0 {
		return 0, errors.New("cooldown must be non-negative")
	}
	return d, nil
}

func Dispatch(ctx context.Context, event Event, notifiers []Notifier) error {
	if len(notifiers) == 0 {
		return errors.New("no notifiers configured")
	}

	errs := make([]error, 0)
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}

	for _, notifier := range notifiers {
		n := notifier
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := n.Notify(ctx, event); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", n.Name(), err))
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func BuildNotifiers(cfg config.NotifyConfig, only []string) ([]Notifier, error) {
	filter := toSet(only)
	notifiers := make([]Notifier, 0, len(cfg.Backends))

	for _, backend := range cfg.Backends {
		name := backendName(backend)
		if len(filter) > 0 {
			if _, byName := filter[name]; !byName {
				if _, byType := filter[strings.TrimSpace(backend.Type)]; !byType {
					continue
				}
			}
		}

		notifier, err := newNotifier(backend)
		if err != nil {
			return nil, err
		}
		notifiers = append(notifiers, notifier)
	}

	if len(notifiers) == 0 {
		return nil, errors.New("no notifier backends matched")
	}

	return notifiers, nil
}

func backendName(backend config.NotifyBackendConfig) string {
	if name := strings.TrimSpace(backend.Name); name != "" {
		return name
	}
	return strings.TrimSpace(backend.Type)
}

func newNotifier(backend config.NotifyBackendConfig) (Notifier, error) {
	timeout := 5 * time.Second
	if strings.TrimSpace(backend.Timeout) != "" {
		parsed, err := time.ParseDuration(backend.Timeout)
		if err != nil {
			return nil, fmt.Errorf("backend %q timeout: %w", backendName(backend), err)
		}
		timeout = parsed
	}

	switch strings.TrimSpace(backend.Type) {
	case "command":
		return &commandNotifier{name: backendName(backend), command: backend.Command, timeout: timeout}, nil
	case "webhook":
		return &webhookNotifier{
			name:   backendName(backend),
			url:    backend.URL,
			client: &http.Client{Timeout: timeout},
		}, nil
	case "ntfy":
		baseURL := strings.TrimRight(strings.TrimSpace(backend.URL), "/")
		if baseURL == "" {
			baseURL = "https://ntfy.sh"
		}
		return &ntfyNotifier{
			name:   backendName(backend),
			url:    baseURL + "/" + strings.TrimLeft(strings.TrimSpace(backend.Topic), "/"),
			client: &http.Client{Timeout: timeout},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported backend type %q", backend.Type)
	}
}

type commandNotifier struct {
	name    string
	command string
	timeout time.Duration
}

func (n *commandNotifier) Name() string { return n.name }

func (n *commandNotifier) Notify(ctx context.Context, event Event) error {
	cmdCtx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", n.command)
	cmd.Env = append(sortedEnviron(),
		"HEALTHD_EVENT_CHECK="+event.CheckName,
		"HEALTHD_EVENT_GROUP="+event.Group,
		"HEALTHD_EVENT_STATE="+string(event.State),
		"HEALTHD_EVENT_PREVIOUS="+string(event.Previous),
		"HEALTHD_EVENT_REASON="+event.Reason,
		"HEALTHD_EVENT_EXIT_CODE="+fmt.Sprintf("%d", event.ExitCode),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

type webhookNotifier struct {
	name   string
	url    string
	client *http.Client
}

func (n *webhookNotifier) Name() string { return n.name }

func (n *webhookNotifier) Notify(ctx context.Context, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	return nil
}

type ntfyNotifier struct {
	name   string
	url    string
	client *http.Client
}

func (n *ntfyNotifier) Name() string { return n.name }

func (n *ntfyNotifier) Notify(ctx context.Context, event Event) error {
	message := fmt.Sprintf("%s -> %s (%s)", event.CheckName, event.State, event.Reason)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("build ntfy request: %w", err)
	}
	req.Header.Set("Title", "healthd alert")
	req.Header.Set("Priority", "default")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send ntfy message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("ntfy status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	return nil
}

func stateForResult(result runner.CheckResult) State {
	if result.Passed {
		return StateOK
	}
	if result.TimedOut || result.ExitCode != 0 {
		return StateCrit
	}
	return StateWarn
}

func sortedEnviron() []string {
	env := os.Environ()
	sort.Strings(env)
	return env
}

func toSet(values []string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}
