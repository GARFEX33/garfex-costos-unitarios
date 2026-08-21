package core

import "context"

// Operation names a logical operation for a diagnostic record.
type Operation string

// DiagnosticRecord captures the opaque technical detail behind the neutral
// error boundary. It is never attached to a public Error value.
type DiagnosticRecord struct {
	Op    Operation
	Kind  string
	ID    int64
	Actor string // caller-supplied, ctx-carried; never persisted, never public
	Cause error
}

// actorContextKey is the unexported context key type for caller attribution.
type actorContextKey struct{}

// WithActor returns a child ctx carrying the caller-supplied actor for
// diagnostic attribution only. It is never business data and is never
// persisted. A blank actor returns ctx unchanged.
func WithActor(ctx context.Context, actor string) context.Context {
	if actor == "" {
		return ctx
	}
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// ActorFrom returns the actor carried by ctx, or "" when absent.
func ActorFrom(ctx context.Context) string {
	actor, _ := ctx.Value(actorContextKey{}).(string)
	return actor
}

// NewDiagnosticRecord assembles the complete diagnostic shape, including the
// ctx-carried actor, so a sink cannot forget attribution.
func NewDiagnosticRecord(ctx context.Context, op Operation, kind string, id int64, cause error) DiagnosticRecord {
	return DiagnosticRecord{Op: op, Kind: kind, ID: id, Actor: ActorFrom(ctx), Cause: cause}
}

// DiagnosticSink records technical causes behind the neutral boundary.
type DiagnosticSink interface {
	Record(ctx context.Context, op Operation, kind string, id int64, cause error)
}

var currentSink DiagnosticSink = noopSink{}

type noopSink struct{}

func (noopSink) Record(context.Context, Operation, string, int64, error) {}

// SetSink replaces the global diagnostic sink; nil restores the no-op sink.
func SetSink(s DiagnosticSink) {
	if s == nil {
		currentSink = noopSink{}
		return
	}
	currentSink = s
}

// Record sends cause to the current diagnostic sink. Nil causes are ignored.
func Record(ctx context.Context, op Operation, kind string, id int64, cause error) {
	if cause == nil {
		return
	}
	currentSink.Record(ctx, op, kind, id, cause)
}

// MapWithDiagnostic records cause and returns the neutral GARFEX mapping.
func MapWithDiagnostic(ctx context.Context, op Operation, err error) Error {
	Record(ctx, op, "", 0, err)
	return Map(err)
}
