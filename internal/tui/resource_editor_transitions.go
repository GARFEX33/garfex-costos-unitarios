package tui

import "context"

// respondToEditor routes one interaction through the editor state machine.
func (a *ResourcesWorkspaceAdapter) respondToEditor(ctx context.Context, input InteractionInput) (InteractionResponse, bool) {
	if input.Kind == InputCancel {
		mode := a.editor.mode
		a.editor = nil
		if mode == editorModeEdit || mode == editorModeDuplicate {
			return a.detailOriginResponse(TextMessage{Text: "Se canceló la operación."}), true
		}
		return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: "Se canceló la operación."}}}, true
	}
	if input.Key != resourceEditorKey {
		return InteractionResponse{}, false
	}

	state := a.editor
	switch state.step {
	case editorStepClase:
		return a.answerClass(input.Value), true
	case editorStepFamily:
		state.family = input.Value
		state.step = editorStepType
		return a.typeQuestion(), true
	case editorStepType:
		state.itemType = input.Value
		state.attributes = a.catalog.AttributesFor(domainScope(state))
		state.nextIndex = 0
		state.step = editorStepAttribute
		return a.advanceEditor(), true
	case editorStepAttribute:
		return a.answerAttribute(input.Value), true
	case editorStepAttributePicker:
		return a.answerAttributePicker(input.Value), true
	case editorStepAttributeEdit:
		return a.answerSingleAttributeEdit(input.Value), true
	case editorStepUnit:
		if state.mode == editorModeEdit || state.mode == editorModeDuplicate {
			state.currentUnit = input.Value
			state.editedCodes = appendUnique(state.editedCodes, editNaturalUnitFieldCode)
			return a.attributePickerQuestion(), true
		}
		return a.finishEditor(ctx, input.Value), true
	case editorStepConfirm:
		return a.answerEditConfirmation(ctx, input.Value), true
	}
	return InteractionResponse{}, true
}
