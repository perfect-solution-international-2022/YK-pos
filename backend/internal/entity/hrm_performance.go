package entity

import (
	"time"

	"github.com/google/uuid"
)

const (
	AnnouncementStatusDraft     = "draft"
	AnnouncementStatusPublished = "published"
	AnnouncementStatusArchived  = "archived"
)

// PerformanceReview is one monthly evaluation. Unique on
// (employee_id, period_year, period_month): saving the form again edits the
// existing review rather than adding a second one, or an employee's average
// rating would depend on how many times their manager clicked save.
//
// Ratings are 1.00-5.00 with halves allowed. They are rates, not currency, so
// they are plain floats rather than integer cents.
type PerformanceReview struct {
	IDMixin
	Timestamps

	BusinessID  uuid.UUID `gorm:"column:business_id;not null"`
	EmployeeID  uuid.UUID `gorm:"column:employee_id;not null"`
	PeriodYear  int       `gorm:"column:period_year;not null"`
	PeriodMonth int       `gorm:"column:period_month;not null"`

	Rating float64 `gorm:"column:rating;not null"`
	// Optional sub-scores on the same scale, absent when a shop grades on the
	// overall rating alone.
	Punctuality  *float64 `gorm:"column:punctuality"`
	Teamwork     *float64 `gorm:"column:teamwork"`
	Productivity *float64 `gorm:"column:productivity"`

	ManagerNotes *string    `gorm:"column:manager_notes"`
	ReviewerID   *uuid.UUID `gorm:"column:reviewer_id"`

	Employee *Employee `gorm:"foreignKey:EmployeeID;references:ID"`
}

func (PerformanceReview) TableName() string { return "hrm_performance_reviews" }

// Announcement is a notice to staff. PublishDate is the day it becomes
// current — a calendar day, not a timestamp, because "from Monday" is what a
// noticeboard means.
type Announcement struct {
	IDMixin
	Timestamps
	SoftDelete

	BusinessID  uuid.UUID  `gorm:"column:business_id;not null"`
	Title       string     `gorm:"column:title;not null"`
	Description string     `gorm:"column:description;not null"`
	PublishDate time.Time  `gorm:"column:publish_date;type:date;not null"`
	ExpiresOn   *time.Time `gorm:"column:expires_on;type:date"`
	Status      string     `gorm:"column:status;not null"`
	CreatedBy   *uuid.UUID `gorm:"column:created_by"`
}

func (Announcement) TableName() string { return "hrm_announcements" }
