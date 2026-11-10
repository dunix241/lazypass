package vault

import "lazypass/internal/vault"

type createAction struct {
	label     string
	kind      vault.EntryType
	container bool
	bulk      bool
}

func (s *StoreForm) createActions() []createAction {
	var actions []createAction
	for _, kind := range s.deps.Service.SupportedEntryTypes() {
		actions = append(actions, createAction{label: kind.String(), kind: kind})
	}
	label, supported := "Folder", false
	if provider, ok := s.deps.Service.Provider.(vault.ContainerActions); ok {
		label, supported = provider.ContainerAction(s.createParent)
	} else {
		_, supported = s.deps.Service.Provider.(vault.FolderCreator)
	}
	if supported {
		actions = append(actions, createAction{label: label, container: true})
	}
	actions = append(actions, createAction{label: "Import authenticators", bulk: true})
	return actions
}

func (s *StoreForm) chooseCreateAction() {
	actions := s.createActions()
	if len(actions) == 0 {
		return
	}
	action := actions[min(s.actionIndex, len(actions)-1)]
	if action.bulk {
		s.choosingType = false
		s.bulk = &bulkImport{phase: "images"}
		return
	}
	s.entryType, s.creatingContainer, s.choosingType = action.kind, action.container, false
}
