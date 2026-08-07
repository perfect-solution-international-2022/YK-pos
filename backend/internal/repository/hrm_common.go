package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

/*
ListParams is the paging/sorting shape every HRM list read takes.

Sort is interpolated into ORDER BY, which GORM does not escape, so it must
already have been validated against a whitelist by the caller (pagination.Parse
plus the endpoint's dto.*SortFields). Nothing in this package re-checks it;
treat an unvalidated value reaching this struct as SQL injection.

Search is likewise already LIKE-escaped by pagination.Parse — the repositories
wrap it in %…% and nothing more.
*/
type ListParams struct {
	Limit  int
	Offset int
	Sort   string
	Order  string // "asc" | "desc"
	Search string
}

/*
DateRange bounds a query on a date column. A zero end of the range means
unbounded, so the same struct serves "everything since March" and "March only".
*/
type DateRange struct {
	From time.Time
	To   time.Time
}

/*
paginate applies ordering, limit and offset in one place, so every HRM list
sorts the same way and pages consistently.

The id tie-break is load-bearing rather than cosmetic: two rows sharing a sort
value (same joining_date, same work_date) would otherwise be ordered
arbitrarily by Postgres, and a row could appear on two pages while another
appeared on none.
*/
func paginate(query *gorm.DB, params ListParams) *gorm.DB {
	if params.Sort != "" {
		query = query.Order(clause.OrderByColumn{
			Column: clause.Column{Name: params.Sort},
			Desc:   params.Order == "desc",
		})
	}
	return query.Order("id").Limit(params.Limit).Offset(params.Offset)
}

/*
likePattern wraps an already-escaped search term for a contains match. Empty in,
empty out, so callers can pass it straight through without a nil check.
*/
func likePattern(search string) string {
	if search == "" {
		return ""
	}
	return "%" + search + "%"
}

/*
applyDateRange filters a query to a bounded window on column. Used by
attendance, leave and the reports, all of which slice by a business date rather
than by created_at.
*/
func applyDateRange(query *gorm.DB, column string, r DateRange) *gorm.DB {
	if !r.From.IsZero() {
		query = query.Where(column+" >= ?", r.From)
	}
	if !r.To.IsZero() {
		query = query.Where(column+" <= ?", r.To)
	}
	return query
}
