package id

// The ID kinds and their prefixes. Prefixes are part of the public API: once
// v1 ships they never change. Add a kind here when its module arrives, and add
// it to the list in kinds_test.go so uniqueness is checked.
const (
	prefixCustomer     = "cus"
	prefixSubscription = "sub"
	prefixInvoice      = "inv"
	prefixPayment      = "pay"
	prefixMandate      = "man"
	prefixPlan         = "pln"
	prefixPrice        = "prc"
	prefixCoupon       = "cpn"
)

// Kinds. Use them as type arguments, for example id.New[id.Customer]().
type (
	// Customer is the kind of CustomerID.
	Customer struct{}
	// Subscription is the kind of SubscriptionID.
	Subscription struct{}
	// Invoice is the kind of InvoiceID.
	Invoice struct{}
	// Payment is the kind of PaymentID.
	Payment struct{}
	// Mandate is the kind of MandateID (a UPI AutoPay or e-NACH mandate).
	Mandate struct{}
	// Plan is the kind of PlanID.
	Plan struct{}
	// Price is the kind of PriceID.
	Price struct{}
	// Coupon is the kind of CouponID.
	Coupon struct{}
)

func (Customer) prefix() string     { return prefixCustomer }
func (Subscription) prefix() string { return prefixSubscription }
func (Invoice) prefix() string      { return prefixInvoice }
func (Payment) prefix() string      { return prefixPayment }
func (Mandate) prefix() string      { return prefixMandate }
func (Plan) prefix() string         { return prefixPlan }
func (Price) prefix() string        { return prefixPrice }
func (Coupon) prefix() string       { return prefixCoupon }

// Typed IDs.
type (
	// CustomerID identifies a customer: cus_...
	CustomerID = ID[Customer]
	// SubscriptionID identifies a subscription: sub_...
	SubscriptionID = ID[Subscription]
	// InvoiceID identifies an invoice: inv_...
	InvoiceID = ID[Invoice]
	// PaymentID identifies a payment attempt: pay_...
	PaymentID = ID[Payment]
	// MandateID identifies a recurring-payment mandate: man_...
	MandateID = ID[Mandate]
	// PlanID identifies a plan: pln_...
	PlanID = ID[Plan]
	// PriceID identifies a price on a plan: prc_...
	PriceID = ID[Price]
	// CouponID identifies a coupon: cpn_...
	CouponID = ID[Coupon]
)
