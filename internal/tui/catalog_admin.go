package tui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// catalogEditorKey identifies every QuestionRequest the catalog-structure
// create/edit engine emits — mirrors resource_editor.go's resourceEditorKey
// (one shared Key per in-progress flow, so respondToEditor can claim any
// answer directed at it without tracking a separate key per field).
const catalogEditorKey = "catalog-editor"

type catalogEditorMode int

const (
	catalogEditorCreate catalogEditorMode = iota
	catalogEditorEdit
)

// catalogLister/catalogGetter/catalogRecordCreator/catalogRecordUpdater are
// the minimal surfaces the generic catalog-admin engine needs from the
// application service — satisfied structurally by *catalogo.Service, the
// same narrow-interface convention resource_editor.go already established
// (resourceSearcher/resourceCreator/...) so this package never imports
// internal/app/catalogo directly.
type catalogLister interface {
	List(ctx context.Context, kind domain.CatalogKindCode, filter domain.CatalogFilter) ([]domain.CatalogRecord, error)
}

type catalogGetter interface {
	Get(ctx context.Context, kind domain.CatalogKindCode, id int64) (domain.CatalogRecord, error)
}

type catalogRecordCreator interface {
	Create(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error)
}

type catalogRecordUpdater interface {
	Update(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (domain.CatalogRecord, error)
}

// catalogEditorState is the in-progress create/edit flow for ONE record of
// ONE CatalogKind — the catalog-structure-admin counterpart to
// resourceEditorState (design D15: a PARALLEL state machine sharing zero
// code with it, since catalog-structure concepts like FieldDescriptor/
// CatalogRecord have no resource-*instance* meaning). Entirely
// descriptor-driven: nothing here ever branches on a specific
// CatalogKindCode — the SAME functions below serve every registered kind,
// differing only by def's own data (spec: Generic Descriptor-Driven Engine).
type catalogEditorState struct {
	mode catalogEditorMode
	// def is the CatalogKind descriptor this flow renders from.
	def domain.CatalogKind
	// step indexes def.Fields for the field currently being asked. Fields
	// are always walked in the registry's own declared order, which already
	// places parent-scoping refs before the fields they narrow (e.g. Tipo's
	// "class" then "family" before "code") — see catalog_kind.go.
	step int
	// values accumulates one CatalogValue per answered field, keyed by
	// FieldDescriptor.Name. Seeded from the existing record's Values for
	// catalogEditorEdit, empty for catalogEditorCreate.
	values map[string]domain.CatalogValue
	// id is the repository-assigned CatalogRecord.ID — 0 for create, the
	// existing record's ID for edit (finishEditor's Update target).
	id int64
}

// startCreateFlow begins the "crear" flow for kind — the generic entry point
// every registered CatalogKind shares. Two structurally different kinds
// (e.g. KindClass, a top-level kind, and KindFamily, a child kind scoped by
// a parent ref) both flow through this exact function, fieldQuestion, and
// answerField below — never a per-kind branch (spec: Generic
// Descriptor-Driven Engine, "shared engine path across two kinds").
func (a *CatalogAdminAdapter) startCreateFlow(ctx context.Context, kind domain.CatalogKindCode) (InteractionResponse, error) {
	def, ok := a.registry.Kind(kind)
	if !ok {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No reconozco ese tipo de catálogo."},
		}}, nil
	}
	a.editor = &catalogEditorState{mode: catalogEditorCreate, def: def, values: map[string]domain.CatalogValue{}}
	response, err := a.fieldQuestion(ctx, def.Fields[a.editor.step])
	if err != nil {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude iniciar la creación. Probá de nuevo en un momento."},
		}}, nil
	}
	return response, nil
}

// startEditFlow begins the "editar" flow for an existing record — the same
// generic entry point as startCreateFlow, differing only in the seeded
// values/id and the mode finishEditor later reads to call Update instead of
// Create.
func (a *CatalogAdminAdapter) startEditFlow(ctx context.Context, kind domain.CatalogKindCode, rec domain.CatalogRecord) (InteractionResponse, error) {
	def, ok := a.registry.Kind(kind)
	if !ok {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No reconozco ese tipo de catálogo."},
		}}, nil
	}
	a.editor = &catalogEditorState{mode: catalogEditorEdit, def: def, values: cloneCatalogEditorValues(rec.Values), id: rec.ID}
	response, err := a.fieldQuestion(ctx, def.Fields[a.editor.step])
	if err != nil {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude iniciar la edición. Probá de nuevo en un momento."},
		}}, nil
	}
	return response, nil
}

// respondToEditor handles one InteractionInput while a.editor is
// in-progress — mirrors resource_editor.go's respondToEditor: the returned
// bool reports whether input actually belonged to the editor (its own Key,
// or a cancellation); when false, Respond falls through to its normal
// dispatch instead of this method silently swallowing unrelated input.
func (a *CatalogAdminAdapter) respondToEditor(ctx context.Context, input InteractionInput) (InteractionResponse, bool) {
	if input.Kind == InputCancel {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			TextMessage{Text: "Se canceló la operación."},
		}}, true
	}
	if input.Key != catalogEditorKey {
		return InteractionResponse{}, false
	}
	response, err := a.answerField(ctx, input.Value)
	if err != nil {
		a.editor = nil
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude continuar con la operación. Probá de nuevo en un momento."},
		}}, true
	}
	return response, true
}

// fieldQuestion builds the QuestionRequest for ONE FieldDescriptor — the
// single per-FieldKind rendering switch every registered kind's every field
// goes through (spec: Generic Descriptor-Driven Engine). AllowCreate-flagged
// FieldRef fields emit SelectionSearchable+AllowCustom (design §5/D12) so a
// custom typed answer can later drive a nested create sub-flow — wiring that
// nested sub-flow itself is PR8's job; this PR only emits the correct
// question shape (see buildCatalogFieldValue's FieldRef branch).
func (a *CatalogAdminAdapter) fieldQuestion(ctx context.Context, field domain.FieldDescriptor) (InteractionResponse, error) {
	state := a.editor
	prompt := field.Label
	current, hasCurrent := state.values[field.Name]

	switch field.Kind {
	case domain.FieldRef:
		options, err := a.refOptions(ctx, field, state.values)
		if err != nil {
			return InteractionResponse{}, err
		}
		if len(options) == 0 && !field.AllowCreate {
			refDef, _ := a.registry.Kind(field.RefKind)
			a.editor = nil
			return InteractionResponse{Messages: []InteractionMessage{
				ErrorMessage{Text: fmt.Sprintf("No hay %s disponibles. Primero creá un registro de %s.", strings.ToLower(refDef.Plural), refDef.Singular)},
			}}, nil
		}
		if hasCurrent && current.Ref.Code != "" {
			options = reorderOptionsCurrentFirst(options, current.Ref.Code)
		}
		return InteractionResponse{Pending: QuestionRequest{
			ID: catalogEditorKey, Key: catalogEditorKey, Prompt: prompt, Question: prompt,
			SelectionMode: SelectionSearchable, Options: options, AllowCustom: field.AllowCreate,
		}}, nil

	case domain.FieldEnum:
		options := make([]Option, len(field.EnumValues))
		for i, ev := range field.EnumValues {
			options[i] = Option{ID: ev.Value, Label: ev.Label, Value: ev.Value}
		}
		if hasCurrent {
			options = reorderOptionsCurrentFirst(options, current.Text)
		}
		return InteractionResponse{Pending: QuestionRequest{
			ID: catalogEditorKey, Key: catalogEditorKey, Prompt: prompt, Question: prompt,
			SelectionMode: SelectionSingle, Options: options,
		}}, nil

	case domain.FieldBool:
		options := []Option{
			{ID: "si", Label: "Sí", Value: "true"},
			{ID: "no", Label: "No", Value: "false"},
		}
		if hasCurrent {
			options = reorderOptionsCurrentFirst(options, boolValueString(current.Bool))
		}
		return InteractionResponse{Pending: QuestionRequest{
			ID: catalogEditorKey, Key: catalogEditorKey, Prompt: prompt, Question: prompt,
			SelectionMode: SelectionSingle, Options: options,
		}}, nil

	case domain.FieldText, domain.FieldCode, domain.FieldInt, domain.FieldStringList:
		// Free text: no Options list for CREATE (the user must type a
		// value, mirroring resource_editor.go's QUANTITY question
		// precedent — SelectionSingle+AllowCustom with zero Options, since
		// SelectionFreeText is not wired into model.go's rendering/input
		// handling anywhere in this codebase). EDIT seeds one "keep
		// current" Option so pressing Enter without typing reuses the
		// existing value.
		var options []Option
		if state.mode == catalogEditorEdit {
			if label, value, ok := currentFreeTextOption(field, current, hasCurrent); ok {
				options = []Option{{ID: "actual", Label: label, Value: value}}
			}
		}
		return InteractionResponse{Pending: QuestionRequest{
			ID: catalogEditorKey, Key: catalogEditorKey, Prompt: prompt, Question: prompt,
			SelectionMode: SelectionSingle, Options: options, AllowCustom: true,
		}}, nil

	default:
		return InteractionResponse{}, fmt.Errorf("catalog admin: unsupported field kind %d for field %q", field.Kind, field.Name)
	}
}

// answerField handles the answer to state.def.Fields[state.step], advancing
// to the next field or, once every field has been answered, calling
// finishEditor — the single per-kind-agnostic advance loop every registered
// kind's create/edit flow shares.
func (a *CatalogAdminAdapter) answerField(ctx context.Context, value string) (InteractionResponse, error) {
	state := a.editor
	field := state.def.Fields[state.step]
	newValue, ok, errMsg := buildCatalogFieldValue(field, value)
	if !ok {
		response, err := a.fieldQuestion(ctx, field)
		if err != nil {
			return InteractionResponse{}, err
		}
		response.Messages = []InteractionMessage{ErrorMessage{Text: errMsg}}
		return response, nil
	}
	state.values[field.Name] = newValue
	state.step++
	if state.step >= len(state.def.Fields) {
		return a.finishEditor(ctx)
	}
	return a.fieldQuestion(ctx, state.def.Fields[state.step])
}

// finishEditor builds the CatalogRecord from the fully-answered editor state
// and persists it: Create for catalogEditorCreate, Update (targeting the
// original record's ID) for catalogEditorEdit. It always resets a.editor to
// nil — success and failure both end the flow, mirroring resource_editor.go
// finishEditor's own "simplicity over cleverness" precedent.
func (a *CatalogAdminAdapter) finishEditor(ctx context.Context) (InteractionResponse, error) {
	state := a.editor
	rec := domain.CatalogRecord{Kind: state.def.Code, Values: state.values, Active: true}

	var result domain.CatalogRecord
	var err error
	if state.mode == catalogEditorEdit {
		rec.ID = state.id
		result, err = a.updater.Update(ctx, state.def.Code, rec)
	} else {
		result, err = a.creator.Create(ctx, state.def.Code, rec)
	}

	mode, def := state.mode, state.def
	a.editor = nil
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: catalogErrorMessage(mode, def, err)},
		}}, nil
	}

	verb := "creado"
	if mode == catalogEditorEdit {
		verb = "actualizado"
	}
	title := fmt.Sprintf("%s %s", def.Singular, verb)
	return InteractionResponse{Messages: []InteractionMessage{
		StructuredResult{Title: title, Fields: catalogRecordFields(def, result)},
	}}, nil
}

// refOptions lists field.RefKind's existing records, narrowed by
// field.RefScopedBy (already-answered parent field values, e.g. Tipo's
// "family" field is scoped by its own already-answered "class" value) —
// CatalogFilter.Parent's own documented convention (catalog_record.go).
// Option.Value/ID is always the referenced record's own natural CODE
// (domain D10/CatalogRef — never a numeric database ID), and Option.Label is
// always its Spanish display name (catalogRecordDisplayLabel), never its raw
// Código (spec: Spanish-Only Catalog-Admin UI).
func (a *CatalogAdminAdapter) refOptions(ctx context.Context, field domain.FieldDescriptor, values map[string]domain.CatalogValue) ([]Option, error) {
	filter := domain.CatalogFilter{}
	if len(field.RefScopedBy) > 0 {
		parent := make(map[string]domain.CatalogValue, len(field.RefScopedBy))
		for _, name := range field.RefScopedBy {
			if v, ok := values[name]; ok {
				parent[name] = v
			}
		}
		filter.Parent = parent
	}
	records, err := a.lister.List(ctx, field.RefKind, filter)
	if err != nil {
		return nil, err
	}
	refDef, _ := a.registry.Kind(field.RefKind)
	options := make([]Option, 0, len(records))
	for _, rec := range records {
		code := rec.Values["code"].Text
		if code == "" {
			continue
		}
		options = append(options, Option{ID: code, Label: catalogRecordDisplayLabel(refDef, rec), Value: code})
	}
	return options, nil
}

// cloneCatalogEditorValues returns a shallow copy of values — startEditFlow
// must never let the in-progress editor mutate the CatalogRecord the caller
// (openRecordForEdit) read from the repository.
func cloneCatalogEditorValues(values map[string]domain.CatalogValue) map[string]domain.CatalogValue {
	out := make(map[string]domain.CatalogValue, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}

// buildCatalogFieldValue parses raw (an Option.Value or a free-typed answer)
// into a CatalogValue for field's Kind — the single per-FieldKind parsing
// logic every registered kind's every field shares (mirrors
// resource_editor.go's buildAttributeValue precedent). ok is false when raw
// fails validation (a required empty field, an unparseable integer, an
// unrecognized enum/bool option) — errMsg is the Spanish message to show,
// re-asking the same field rather than silently advancing.
func buildCatalogFieldValue(field domain.FieldDescriptor, raw string) (domain.CatalogValue, bool, string) {
	switch field.Kind {
	case domain.FieldText, domain.FieldCode:
		trimmed := strings.TrimSpace(raw)
		if field.Required && trimmed == "" {
			return domain.CatalogValue{}, false, fmt.Sprintf("%s es obligatorio.", field.Label)
		}
		return domain.CatalogValue{Text: trimmed}, true, ""
	case domain.FieldInt:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			if field.Required {
				return domain.CatalogValue{}, false, fmt.Sprintf("%s es obligatorio.", field.Label)
			}
			return domain.CatalogValue{}, true, ""
		}
		n, err := strconv.Atoi(trimmed)
		if err != nil {
			return domain.CatalogValue{}, false, fmt.Sprintf("%q no es un número entero válido para %s.", raw, field.Label)
		}
		return domain.CatalogValue{Int: n}, true, ""
	case domain.FieldBool:
		switch raw {
		case "true":
			return domain.CatalogValue{Bool: true}, true, ""
		case "false":
			return domain.CatalogValue{Bool: false}, true, ""
		default:
			return domain.CatalogValue{}, false, fmt.Sprintf("Respuesta no válida para %s.", field.Label)
		}
	case domain.FieldStringList:
		var list []string
		for _, part := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				list = append(list, trimmed)
			}
		}
		if field.Required && len(list) == 0 {
			return domain.CatalogValue{}, false, fmt.Sprintf("%s es obligatorio.", field.Label)
		}
		return domain.CatalogValue{List: list}, true, ""
	case domain.FieldEnum:
		for _, ev := range field.EnumValues {
			if ev.Value == raw {
				return domain.CatalogValue{Text: raw}, true, ""
			}
		}
		return domain.CatalogValue{}, false, fmt.Sprintf("Opción no válida para %s.", field.Label)
	case domain.FieldRef:
		trimmed := strings.TrimSpace(raw)
		if field.Required && trimmed == "" {
			return domain.CatalogValue{}, false, fmt.Sprintf("%s es obligatorio.", field.Label)
		}
		return domain.CatalogValue{Ref: domain.CatalogRef{Kind: field.RefKind, Code: trimmed}}, true, ""
	default:
		return domain.CatalogValue{}, false, "Tipo de campo no soportado."
	}
}

// currentFreeTextOption builds the "Mantener actual: X" Option for a
// catalogEditorEdit free-text field (FieldText/FieldCode/FieldInt/
// FieldStringList) — ok is false when there is no meaningful current value
// to offer (e.g. an unset optional field), matching resource_editor.go's
// "never fabricate a default for nothing" precedent.
func currentFreeTextOption(field domain.FieldDescriptor, current domain.CatalogValue, hasCurrent bool) (label, value string, ok bool) {
	if !hasCurrent {
		return "", "", false
	}
	switch field.Kind {
	case domain.FieldInt:
		return fmt.Sprintf("Mantener actual: %d", current.Int), strconv.Itoa(current.Int), true
	case domain.FieldStringList:
		if len(current.List) == 0 {
			return "", "", false
		}
		joined := strings.Join(current.List, ", ")
		return "Mantener actual: " + joined, joined, true
	default: // FieldText, FieldCode
		if current.Text == "" {
			return "", "", false
		}
		return "Mantener actual: " + current.Text, current.Text, true
	}
}

// reorderOptionsCurrentFirst reorders options so the one matching
// currentValue (if any) is first — the catalog-admin engine's own analogue
// of resource_editor.go's defaultOptionFirst/defaultUnitFirst (D15: a
// separate implementation, not shared code, since it operates on []Option
// rather than a domain-specific slice type).
func reorderOptionsCurrentFirst(options []Option, currentValue string) []Option {
	if currentValue == "" {
		return options
	}
	for i, opt := range options {
		if opt.Value != currentValue {
			continue
		}
		if i == 0 {
			return options
		}
		reordered := make([]Option, 0, len(options))
		reordered = append(reordered, opt)
		reordered = append(reordered, options[:i]...)
		reordered = append(reordered, options[i+1:]...)
		return reordered
	}
	return options
}

func boolValueString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// catalogRecordDisplayLabel renders rec's Spanish display label for use as
// an Option.Label/StructuredResult field — the first non-empty candidate
// among name/label/symbol (the Spanish-labeled fields every kind that is
// ever a FieldRef target actually declares), falling back to its Código only
// when no friendlier field exists (e.g. Unidad, which has no separate
// display name). This is the generic engine's own "never render a raw
// Código as if it were a friendly label" guard (spec: Spanish-Only
// Catalog-Admin UI) — it never special-cases a specific CatalogKindCode.
func catalogRecordDisplayLabel(def domain.CatalogKind, rec domain.CatalogRecord) string {
	for _, name := range []string{"name", "label", "symbol"} {
		if v, ok := rec.Values[name]; ok && strings.TrimSpace(v.Text) != "" {
			return v.Text
		}
	}
	if v, ok := rec.Values["code"]; ok && v.Text != "" {
		return v.Text
	}
	return fmt.Sprintf("%s #%d", def.Singular, rec.ID)
}

// catalogRecordFields renders rec as the StructuredResult.Fields shown after
// a successful create/edit — one Field per def.Fields entry, Label always
// the FieldDescriptor's own Spanish Label, never a raw storage key or Código
// (spec: Spanish-Only Catalog-Admin UI).
func catalogRecordFields(def domain.CatalogKind, rec domain.CatalogRecord) []Field {
	fields := make([]Field, 0, len(def.Fields))
	for _, fd := range def.Fields {
		fields = append(fields, Field{Label: fd.Label, Value: formatCatalogFieldValue(fd, rec.Values[fd.Name])})
	}
	return fields
}

// formatCatalogFieldValue renders one CatalogValue for display, per its
// FieldDescriptor.Kind — the catalog-admin counterpart to
// resource_editor.go's formatResourceAttributeValue.
func formatCatalogFieldValue(field domain.FieldDescriptor, v domain.CatalogValue) string {
	switch field.Kind {
	case domain.FieldBool:
		if v.Bool {
			return "Sí"
		}
		return "No"
	case domain.FieldInt:
		return strconv.Itoa(v.Int)
	case domain.FieldStringList:
		return strings.Join(v.List, ", ")
	case domain.FieldRef:
		return v.Ref.Code
	case domain.FieldEnum:
		for _, ev := range field.EnumValues {
			if ev.Value == v.Text {
				return ev.Label
			}
		}
		return v.Text
	default:
		return v.Text
	}
}

// catalogErrorMessage maps a Create/Update error into a Spanish, kind-aware
// message — never leaking a raw Go error or a raw CatalogKindCode into the
// UI (spec: Spanish-Only Catalog-Admin UI). PR7 owns the dedicated
// dependency-guard/Desactivar messaging (design §6/§1a); this is the
// generic Create/Edit engine's own baseline error surface.
func catalogErrorMessage(mode catalogEditorMode, def domain.CatalogKind, err error) string {
	verb := "crear"
	if mode == catalogEditorEdit {
		verb = "guardar"
	}
	switch {
	case errors.Is(err, domain.ErrCodeImmutable):
		return "El código ya está en uso por recursos existentes y no puede cambiarse."
	case errors.Is(err, domain.ErrCatalogDuplicate):
		return fmt.Sprintf("Ya existe un registro de %s con ese código.", def.Singular)
	case errors.Is(err, domain.ErrCatalogReference):
		return "Una de las referencias seleccionadas no es válida."
	case errors.Is(err, domain.ErrResourceValidation):
		reason := strings.TrimPrefix(err.Error(), domain.ErrResourceValidation.Error()+": ")
		return fmt.Sprintf("No pude %s %s: %s.", verb, strings.ToLower(def.Singular), reason)
	case errors.Is(err, domain.ErrResourceReference):
		reason := strings.TrimPrefix(err.Error(), domain.ErrResourceReference.Error()+": ")
		return fmt.Sprintf("No pude %s %s: %s.", verb, strings.ToLower(def.Singular), reason)
	default:
		return fmt.Sprintf("No pude %s %s. Probá de nuevo en un momento.", verb, strings.ToLower(def.Singular))
	}
}
