package tui

import "context"

type Option struct {
	ID       string
	Label    string
	Value    string
	Target   ActionTarget
	Metadata map[string]string
}

type Field struct {
	Label string
	Value string
}

type Action struct {
	ID     string
	Label  string
	Value  string
	Target ActionTarget
}

type ActionTarget string

const (
	ActionTargetAgent ActionTarget = "agent"
	ActionTargetLocal ActionTarget = "local"
)

type TextMessage struct{ Text string }

type QuestionRequest struct {
	ID            string
	Key           string
	Prompt        string
	Question      string
	SelectionMode SelectionMode
	Options       []Option
	AllowCustom   bool
	MinSelections int
	MaxSelections int
	Step          int
	TotalSteps    int
	Metadata      map[string]string
}

type SelectionMode string

const (
	SelectionSingle   SelectionMode = "single"
	SelectionMultiple SelectionMode = "multiple"
	SelectionFreeText SelectionMode = "free-text"
)

type ConfirmationRequest struct {
	ID           string
	Key          string
	Question     string
	ConfirmLabel string
	CancelLabel  string
	Metadata     map[string]string
}

type ActionRequest struct {
	ID       string
	Key      string
	Question string
	Actions  []Action
	Metadata map[string]string
}

type StructuredResult struct {
	Title    string
	Subtitle string
	Fields   []Field
	Sections []Section
	Actions  []Action
}

type Section struct {
	Title  string
	Fields []Field
}

type ErrorMessage struct{ Text string }

type InteractionMessage interface{ InteractionMessage() }

func (TextMessage) InteractionMessage()         {}
func (QuestionRequest) InteractionMessage()     {}
func (ConfirmationRequest) InteractionMessage() {}
func (ActionRequest) InteractionMessage()       {}
func (StructuredResult) InteractionMessage()    {}
func (ErrorMessage) InteractionMessage()        {}

type InteractionResponse struct {
	Messages []InteractionMessage
	Pending  InteractionMessage
}

type InputKind uint8

const (
	InputText InputKind = iota
	InputSelection
	InputAction
	InputCancel
)

type InteractionInput struct {
	Kind     InputKind
	Key      string
	Value    string
	Values   []string
	ActionID string
	Target   ActionTarget
}

type InteractionAgent interface {
	Respond(context.Context, InteractionInput) (InteractionResponse, error)
}

type InteractionEngine struct {
	agent   InteractionAgent
	history []InteractionMessage
	pending InteractionMessage
}

func NewInteractionEngine(agent InteractionAgent) *InteractionEngine {
	return &InteractionEngine{agent: agent}
}

func (e *InteractionEngine) Respond(ctx context.Context, input InteractionInput) InteractionResponse {
	if input.Target == ActionTargetLocal {
		return InteractionResponse{}
	}
	if e.agent == nil {
		response := InteractionResponse{Messages: []InteractionMessage{ErrorMessage{Text: "No hay un agente disponible."}}}
		e.history = append(e.history, response.Messages...)
		e.pending = nil
		return response
	}
	response, err := e.agent.Respond(ctx, input)
	if err != nil {
		response = InteractionResponse{Messages: []InteractionMessage{ErrorMessage{Text: err.Error()}}}
	}
	e.history = append(e.history, response.Messages...)
	e.pending = response.Pending
	return response
}

func (e *InteractionEngine) Text(ctx context.Context, value string) InteractionResponse {
	return e.Respond(ctx, InteractionInput{Kind: InputText, Value: value})
}

func (e *InteractionEngine) Select(ctx context.Context, value string) InteractionResponse {
	return e.Respond(ctx, InteractionInput{Kind: InputSelection, Value: value})
}

func (e *InteractionEngine) Action(ctx context.Context, value string) InteractionResponse {
	return e.Respond(ctx, InteractionInput{Kind: InputAction, Value: value})
}

func (e *InteractionEngine) Cancel(ctx context.Context) InteractionResponse {
	return e.Respond(ctx, InteractionInput{Kind: InputCancel})
}

func (e *InteractionEngine) History() []InteractionMessage {
	return append([]InteractionMessage(nil), e.history...)
}

func (e *InteractionEngine) Pending() InteractionMessage { return e.pending }
