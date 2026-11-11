package vault

import (
	"lazypass/internal/vault"
	"strings"
)

type createAction struct {
	label     string
	kind      vault.EntryType
	container bool
	bulk      bool
}

func (s *StoreForm) createActions() []createAction {
	actions := s.allCreateActions()
	if s.actionFilter == "" {
		return actions
	}
	query := strings.ToLower(s.actionFilter)
	filtered := actions[:0]
	for _, action := range actions {
		if strings.Contains(strings.ToLower(action.label), query) {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func (s *StoreForm) allCreateActions() []createAction {
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
	action := actions[max(0, min(s.actionIndex, len(actions)-1))]
	if action.bulk {
		s.choosingType = false
		s.bulk = &bulkImport{phase: "images"}
		return
	}
	s.entryType, s.creatingContainer, s.choosingType = action.kind, action.container, false
}
