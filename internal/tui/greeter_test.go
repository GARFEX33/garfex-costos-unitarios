package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

type greetingAgent struct{ text string }

func (a *greetingAgent) Greeting() InteractionMessage { return TextMessage{Text: a.text} }

func (a *greetingAgent) Respond(context.Context, InteractionInput) (InteractionResponse, error) {
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: a.text}}}, nil
}

func TestNewWithAgentSeedsGreetingWhenAgentImplementsGreeter(t *testing.T) {
	agent := &greetingAgent{text: "hola desde el agente real"}
	m := NewWithAgent(Handlers{}, agent)
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, agent.text) {
		t.Fatalf("view = %q, want it to contain the seeded greeting %q with zero prior input", plain, agent.text)
	}
}

func TestNewWithAgentFakeAgentHasNoGreeting(t *testing.T) {
	m := NewWithAgent(Handlers{}, NewFakeAgent())
	if len(m.history) != 0 {
		t.Fatalf("history = %v, want empty history (FakeAgent does not implement greeter)", m.history)
	}
}
