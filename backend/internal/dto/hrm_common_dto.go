package dto

import "github.com/SandaruwanWeerawardhana/pos-backend/pkg/response"

/*
ListResponse is the shape every paginated HRM list returns: the page and its
counts as siblings, matching UserListResponse and OrderListResponse.

Generic rather than one named struct per resource, because eleven identical
two-field structs would be eleven places for the JSON keys to drift apart. The
type parameter still gives each endpoint a concrete, documentable response type
(ListResponse[EmployeeResponse]), so nothing is lost on the wire or in the docs.
*/
type ListResponse[T any] struct {
	Items []T           `json:"items"`
	Meta  response.Meta `json:"meta"`
}

/*
NewListResponse builds one, guaranteeing a non-nil Items slice: encoding/json
renders a nil slice as null, and the client calls .map() on the response
directly.
*/
func NewListResponse[T any](items []T, meta response.Meta) ListResponse[T] {
	if items == nil {
		items = []T{}
	}
	return ListResponse[T]{Items: items, Meta: meta}
}

/*
HRMListQuery is the query string every HRM list read accepts. It embeds the
shared PaginationQuery (page, per_page, sort, order, search) and adds the
filters common to the module's history endpoints.

Dates are yyyy-mm-dd rather than epoch millis: these bound business days, and a
report for "March" is a range of calendar dates, not of instants.
*/
type HRMListQuery struct {
	PaginationQuery

	From string `query:"from" validate:"omitempty,datetime=2006-01-02"`
	To   string `query:"to" validate:"omitempty,datetime=2006-01-02"`
}
