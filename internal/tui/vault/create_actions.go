package vault

import "lazypass/internal/vault"

type createAction struct {
	label     string
	kind      vault.EntryType
	container bool
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
	return actions
}

func (s *StoreForm) chooseCreateAction() {
	actions := s.createActions()
	if len(actions) == 0 {
		return
	}
	action := actions[min(s.actionIndex, len(actions)-1)]
	s.entryType, s.creatingContainer, s.choosingType = action.kind, action.container, false
}
