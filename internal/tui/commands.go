package tui

type assistantAction struct {
	id       string
	label    string
	children []assistantAction
}

var assistantActions = []assistantAction{
	{
		id: "materials", label: "Materiales",
		children: []assistantAction{
			{id: "material-search", label: "Buscar material"},
		},
	},
	{id: "concepts", label: "Conceptos"},
	{id: "apu", label: "APU"},
	{id: "suppliers", label: "Proveedores"},
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
