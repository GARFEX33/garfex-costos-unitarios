package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"
)

type SupplierListService interface {
	SearchSuppliers(context.Context, domain.SupplierSearch) ([]domain.Supplier, error)
}

type SupplierDetailService interface {
	GetSupplier(context.Context, int64) (domain.Supplier, error)
	GetBranch(context.Context, int64, int64) (domain.Branch, error)
	GetContact(context.Context, int64, int64) (domain.Contact, error)
	ListBranches(context.Context, int64, domain.ListCriteria) ([]domain.Branch, error)
	ListContacts(context.Context, int64, domain.ContactListCriteria) ([]domain.Contact, error)
}

type SupplierCreateService interface {
	CreateSupplier(context.Context, domain.SupplierDetails) (domain.Supplier, error)
}

type SupplierUpdateService interface {
	UpdateSupplier(context.Context, int64, domain.SupplierDetails) (domain.Supplier, error)
}

type SupplierLifecycleService interface {
	DeactivateSupplier(context.Context, int64) (domain.Supplier, error)
	ReactivateSupplier(context.Context, int64) (domain.Supplier, error)
}

type SupplierChildMutationService interface {
	AddBranch(context.Context, int64, domain.BranchDetails) (domain.Branch, error)
	UpdateBranch(context.Context, int64, int64, domain.BranchDetails) (domain.Branch, error)
	DeactivateBranch(context.Context, int64, int64) (domain.Branch, error)
	ReactivateBranch(context.Context, int64, int64) (domain.Branch, error)
	AddContact(context.Context, int64, domain.ContactDetails) (domain.Contact, error)
	UpdateContact(context.Context, int64, int64, domain.ContactDetails) (domain.Contact, error)
	DeactivateContact(context.Context, int64, int64) (domain.Contact, error)
	ReactivateContact(context.Context, int64, int64) (domain.Contact, error)
}
type SupplierListMsg struct {
	RouteID, RequestID uint64
	Rows               []SupplierRow
	Err                error
}

type SupplierDetailMsg struct {
	RouteID, RequestID uint64
	Detail             SupplierDetail
	Err                error
}

type BranchDetailMsg struct {
	RouteID, RequestID uint64
	Supplier           domain.Supplier
	Branch             domain.Branch
	Contacts           []domain.Contact
	Err                error
}

type ContactDetailMsg struct {
	RouteID, RequestID uint64
	Contact            domain.Contact
	Branch             domain.Branch
	Err                error
}

type SupplierMutationKind uint8

const (
	SupplierMutationCreate SupplierMutationKind = iota
	SupplierMutationUpdate
	SupplierMutationDeactivate
	SupplierMutationReactivate
)

type SupplierMutationMsg struct {
	RouteID, RequestID uint64
	Kind               SupplierMutationKind
	Supplier           domain.Supplier
	Err                error
}

type ChildMutationKind uint8

const (
	ChildMutationCreate ChildMutationKind = iota
	ChildMutationUpdate
	ChildMutationDeactivate
	ChildMutationReactivate
)

type ChildMutationMsg struct {
	RouteID, RequestID  uint64
	Kind                ChildMutationKind
	Contact             bool
	SupplierID, ChildID int64
	Branch              domain.Branch
	ContactValue        domain.Contact
	Err                 error
}

const supplierPageSize = 25

func supplierListCmd(service SupplierListService, frame SupplierManagerFrame) tea.Cmd {
	return func() tea.Msg {
		values, err := service.SearchSuppliers(context.Background(), domain.SupplierSearch{
			Text: frame.Query, Active: supplierFilterActive(frame.Filter), Limit: supplierPageSize, Offset: frame.Offset,
		})
		rows := make([]SupplierRow, len(values))
		for i := range values {
			rows[i] = SupplierRow{ID: values[i].ID}
		}
		return SupplierListMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Rows: rows, Err: err}
	}
}

func supplierDetailCmd(service SupplierDetailService, frame SupplierDetailFrame) tea.Cmd {
	return func() tea.Msg {
		supplier, err := service.GetSupplier(context.Background(), frame.SupplierID)
		if err != nil {
			return SupplierDetailMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Err: err}
		}
		active := true
		branches, err := service.ListBranches(context.Background(), frame.SupplierID, domain.ListCriteria{Active: &active, Limit: supplierPageSize})
		if err != nil {
			return SupplierDetailMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Err: err}
		}
		contacts, err := service.ListContacts(context.Background(), frame.SupplierID, domain.ContactListCriteria{Active: &active, Limit: supplierPageSize})
		return SupplierDetailMsg{
			RouteID: frame.RouteID, RequestID: frame.RequestID,
			Detail: SupplierDetail{Supplier: supplier, Branches: branches, Contacts: contacts}, Err: err,
		}
	}
}

func branchDetailCmd(service SupplierDetailService, frame BranchDetailFrame) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		supplier, err := service.GetSupplier(ctx, frame.SupplierID)
		if err != nil {
			return BranchDetailMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Err: err}
		}
		branch, err := service.GetBranch(ctx, frame.SupplierID, frame.BranchID)
		if err != nil {
			return BranchDetailMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Supplier: supplier, Err: err}
		}
		active := true
		contacts, err := service.ListContacts(ctx, frame.SupplierID, domain.ContactListCriteria{Active: &active, BranchID: &frame.BranchID, Limit: supplierPageSize})
		return BranchDetailMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Supplier: supplier, Branch: branch, Contacts: contacts, Err: err}
	}
}

func contactDetailCmd(service SupplierDetailService, frame ContactDetailFrame) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		contact, err := service.GetContact(ctx, frame.SupplierID, frame.ContactID)
		if err != nil {
			return ContactDetailMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Err: err}
		}
		var branch domain.Branch
		if contact.BranchID != nil {
			branch, err = service.GetBranch(ctx, frame.SupplierID, *contact.BranchID)
		}
		return ContactDetailMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Contact: contact, Branch: branch, Err: err}
	}
}

func supplierCreateCmd(service SupplierCreateService, frame SupplierEditFrame) tea.Cmd {
	return func() tea.Msg {
		supplier, err := service.CreateSupplier(context.Background(), frame.Values)
		return SupplierMutationMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Kind: SupplierMutationCreate, Supplier: supplier, Err: err}
	}
}

func supplierUpdateCmd(service SupplierUpdateService, frame SupplierEditFrame) tea.Cmd {
	return func() tea.Msg {
		supplier, err := service.UpdateSupplier(context.Background(), frame.SupplierID, frame.Values)
		return SupplierMutationMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Kind: SupplierMutationUpdate, Supplier: supplier, Err: err}
	}
}

func supplierLifecycleCmd(service SupplierLifecycleService, frame SupplierLifecycleFrame) tea.Cmd {
	return func() tea.Msg {
		var (
			supplier domain.Supplier
			err      error
		)
		if frame.Active {
			supplier, err = service.DeactivateSupplier(context.Background(), frame.SupplierID)
		} else {
			supplier, err = service.ReactivateSupplier(context.Background(), frame.SupplierID)
		}
		kind := SupplierMutationDeactivate
		if !frame.Active {
			kind = SupplierMutationReactivate
		}
		return SupplierMutationMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Kind: kind, Supplier: supplier, Err: err}
	}
}

func childMutationCmd(service any, frame any) tea.Cmd {
	mutationService, ok := service.(SupplierChildMutationService)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		switch value := frame.(type) {
		case BranchEditFrame:
			if value.Mode {
				branch, err := mutationService.UpdateBranch(ctx, value.SupplierID, value.BranchID, value.Values)
				return childBranchMutation(value, ChildMutationUpdate, branch, err)
			}
			branch, err := mutationService.AddBranch(ctx, value.SupplierID, value.Values)
			return childBranchMutation(value, ChildMutationCreate, branch, err)
		case ContactEditFrame:
			if value.Mode {
				contact, err := mutationService.UpdateContact(ctx, value.SupplierID, value.ContactID, value.Values)
				return childContactMutation(value, ChildMutationUpdate, contact, err)
			}
			contact, err := mutationService.AddContact(ctx, value.SupplierID, value.Values)
			return childContactMutation(value, ChildMutationCreate, contact, err)
		case ChildLifecycleFrame:
			kind := ChildMutationDeactivate
			if !value.Active {
				kind = ChildMutationReactivate
			}
			if value.Contact {
				if value.Active {
					contact, err := mutationService.DeactivateContact(ctx, value.SupplierID, value.ChildID)
					return childContactLifecycleMutation(value, kind, contact, err)
				}
				contact, err := mutationService.ReactivateContact(ctx, value.SupplierID, value.ChildID)
				return childContactLifecycleMutation(value, kind, contact, err)
			}
			if value.Active {
				branch, err := mutationService.DeactivateBranch(ctx, value.SupplierID, value.ChildID)
				return childBranchLifecycleMutation(value, kind, branch, err)
			}
			branch, err := mutationService.ReactivateBranch(ctx, value.SupplierID, value.ChildID)
			return childBranchLifecycleMutation(value, kind, branch, err)
		default:
			return ChildMutationMsg{}
		}
	}
}

func childBranchMutation(f BranchEditFrame, kind ChildMutationKind, value domain.Branch, err error) tea.Msg {
	return ChildMutationMsg{RouteID: f.RouteID, RequestID: f.RequestID, Kind: kind, SupplierID: f.SupplierID, ChildID: childMutationReplyID(f.BranchID, value.ID, err), Branch: value, Err: err}
}

func childContactMutation(f ContactEditFrame, kind ChildMutationKind, value domain.Contact, err error) tea.Msg {
	return ChildMutationMsg{RouteID: f.RouteID, RequestID: f.RequestID, Kind: kind, Contact: true, SupplierID: f.SupplierID, ChildID: childMutationReplyID(f.ContactID, value.ID, err), ContactValue: value, Err: err}
}

func childMutationReplyID(requested, returned int64, err error) int64 {
	return map[bool]int64{true: requested, false: returned}[err != nil]
}

func childBranchLifecycleMutation(f ChildLifecycleFrame, kind ChildMutationKind, value domain.Branch, err error) tea.Msg {
	return ChildMutationMsg{RouteID: f.RouteID, RequestID: f.RequestID, Kind: kind, SupplierID: f.SupplierID, ChildID: f.ChildID, Branch: value, Err: err}
}

func childContactLifecycleMutation(f ChildLifecycleFrame, kind ChildMutationKind, value domain.Contact, err error) tea.Msg {
	return ChildMutationMsg{RouteID: f.RouteID, RequestID: f.RequestID, Kind: kind, Contact: true, SupplierID: f.SupplierID, ChildID: f.ChildID, ContactValue: value, Err: err}
}

func childMutationIdentity(frame any) (uint64, uint64, bool, int64, int64) {
	switch value := frame.(type) {
	case BranchEditFrame:
		return value.RouteID, value.RequestID, false, value.SupplierID, value.BranchID
	case ContactEditFrame:
		return value.RouteID, value.RequestID, true, value.SupplierID, value.ContactID
	case ChildLifecycleFrame:
		return value.RouteID, value.RequestID, value.Contact, value.SupplierID, value.ChildID
	default:
		return 0, 0, false, 0, 0
	}
}
