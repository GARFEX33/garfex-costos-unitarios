package tui

type assistantAction struct {
	id       string
	label    string
	children []assistantAction
}

var assistantActions = []assistantAction{
	{id: "materials", label: "Materiales Maestros"},
	{id: "concepts", label: "Conceptos"},
	{id: "apu", label: "APU"},
	{id: "suppliers", label: "Proveedores"},
}

// materialsActions is the "/" palette's action tree while inside the
// Materiales Maestros workspace (see Model.activePaletteActions) — scoped
// to that workspace instead of the global assistantActions tree.
//
// createResourceActionID (not a "materials"-named constant) is intentional:
// it is the SAME action ID ResourcesWorkspaceAdapter.Respond checks for
// every workspace slot (see resources_workspace_dispatch.go and design §7's
// workspaceActions), not a Materials-specific one — this tree is simply the
// one slot wired up so far pending PR8's full commands.go rewrite.
var materialsActions = []assistantAction{
	{id: createResourceActionID, label: "Crear material"},
}

func flattenLeafActions(actions []assistantAction) []assistantAction {
	var result []assistantAction
	for _, action := range actions {
		if len(action.children) > 0 {
			result = append(result, flattenLeafActions(action.children)...)
			continue
		}
		result = append(result, action)
	}
	return result
}

func actionOptions(actions []assistantAction) []Option {
	options := make([]Option, 0, len(actions))
	for _, action := range actions {
		options = append(options, Option{
			ID: action.id, Label: action.label, Value: action.id,
		})
	}
	return options
}

func manualMaterialOptions() []Option {
	return []Option{
		{Label: "THW-LS 10 AWG Negro", Value: "thw-ls-10-black"},
		{Label: "THW-LS 12 AWG Blanco", Value: "thw-ls-12-white"},
		{Label: "XHHW-2 10 AWG Negro", Value: "xhhw-2-10-black"},
	}
}
