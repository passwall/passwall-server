package domain

import (
	"time"
)

// InvoiceStatus represents the status of an invoice (from Stripe)
type InvoiceStatus string

const (
	InvoiceStatusDraft         InvoiceStatus = "draft"
	InvoiceStatusOpen          InvoiceStatus = "open"
	InvoiceStatusPaid          InvoiceStatus = "paid"
	InvoiceStatusVoid          InvoiceStatus = "void"
	InvoiceStatusUncollectible InvoiceStatus = "uncollectible"
)

// String returns the string representation of InvoiceStatus
func (i InvoiceStatus) String() string {
	return string(i)
}

// InvoiceDTO represents invoice data fetched from Stripe (no DB table)
// Invoices are fetched directly from Stripe API, not stored in database
type InvoiceDTO struct {
	ID               uint          `json:"id,omitempty"`              // Not used (no DB)
	UUID             string        `json:"uuid,omitempty"`            // Not used (no DB)
	SubscriptionID   uint          `json:"subscription_id,omitempty"` // Not used (no DB)
	Status           InvoiceStatus `json:"status"`
	AmountCents      int           `json:"amount_cents"`
	AmountDisplay    string        `json:"amount_display"`
	Currency         string        `json:"currency"`
	IssuedAt         time.Time     `json:"issued_at"`
	PaidAt           *time.Time    `json:"paid_at,omitempty"`
	DueDate          *time.Time    `json:"due_date,omitempty"`
	StripeInvoiceID  *string       `json:"stripe_invoice_id,omitempty"`
	InvoicePDFURL    *string       `json:"invoice_pdf_url,omitempty"`
	HostedInvoiceURL *string       `json:"hosted_invoice_url,omitempty"`
	CreatedAt        time.Time     `json:"created_at,omitempty"` // Not used (no DB)
}
