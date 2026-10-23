package httpx

import "context"

// Event is a fixed operational signal, never an application or user label.
type Event uint8

const (
	AdmissionOverload Event = iota + 1
	RateLimited
	RateCapacity
	StaleAction
	AuthorizationRejected
	CommittedEffectFailure
	ResponseAborted
	IdempotencyCapacity
	IdempotencyWaitCapacity
	RevocationCapacity
	CommandOverload
)

func (event Event) String() string {
	switch event {
	case AdmissionOverload:
		return "admission_overload"
	case RateLimited:
		return "rate_limited"
	case RateCapacity:
		return "rate_capacity"
	case StaleAction:
		return "stale_action"
	case AuthorizationRejected:
		return "authorization_rejected"
	case CommittedEffectFailure:
		return "committed_effect_failure"
	case ResponseAborted:
		return "response_aborted"
	case IdempotencyCapacity:
		return "idempotency_capacity"
	case IdempotencyWaitCapacity:
		return "idempotency_wait_capacity"
	case RevocationCapacity:
		return "revocation_capacity"
	case CommandOverload:
		return "command_overload"
	default:
		return ""
	}
}

type eventObserverKey struct{}

// WithEventObserver attaches a concurrency-safe operational sink. Observers
// must not panic. Existing observers are retained when collectors are nested.
func WithEventObserver(ctx context.Context, observer func(Event)) context.Context {
	if observer == nil {
		return ctx
	}
	if previous, ok := ctx.Value(eventObserverKey{}).(func(Event)); ok && previous != nil {
		next := observer
		observer = func(event Event) {
			previous(event)
			next(event)
		}
	}
	return context.WithValue(ctx, eventObserverKey{}, observer)
}

// Observe records a known event when a collector is installed. It retains no
// request state and has no effect on response status or application execution.
func Observe(ctx context.Context, event Event) {
	if event.String() == "" {
		return
	}
	if observer, ok := ctx.Value(eventObserverKey{}).(func(Event)); ok && observer != nil {
		observer(event)
	}
}
