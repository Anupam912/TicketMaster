package services

import (
	"context"
	"errors"
	"fmt"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/models"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/refund"
)

// Sentinel errors for payment operations.
var (
	ErrPaymentFailed       = errors.New("payment processing failed")
	ErrPaymentDeclined     = errors.New("payment was declined")
	ErrRefundFailed        = errors.New("refund processing failed")
	ErrStripeNotConfigured = errors.New("stripe is not configured")
	ErrPaymentIntentMissing = errors.New("payment intent is not prepared")
)

// PaymentResult contains the result of a payment operation.
type PaymentResult struct {
	PaymentID    string  `json:"payment_id"`
	Status       string  `json:"status"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	ClientSecret string  `json:"client_secret,omitempty"`
}

// PaymentService handles payment processing via Stripe.
type PaymentService struct {
	config    *config.Config
	isEnabled bool
}

// NewPaymentService creates a new PaymentService instance.
func NewPaymentService(cfg *config.Config) *PaymentService {
	ps := &PaymentService{
		config:    cfg,
		isEnabled: false,
	}

	if cfg.Stripe.SecretKey != "" {
		stripe.Key = cfg.Stripe.SecretKey
		ps.isEnabled = true
	}

	return ps
}

// IsEnabled returns whether Stripe payment processing is configured.
func (s *PaymentService) IsEnabled() bool {
	return s.isEnabled
}

// CreatePaymentIntent creates a Stripe PaymentIntent for the booking.
// This is used for client-side payment confirmation (recommended approach).
func (s *PaymentService) CreatePaymentIntent(ctx context.Context, booking *models.Booking, userEmail string) (*PaymentResult, error) {
	if !s.isEnabled {
		return s.simulatePayment(booking)
	}

	params := s.newPaymentIntentParams(booking)
	if userEmail != "" {
		params.ReceiptEmail = stripe.String(userEmail)
	}
	params.SetIdempotencyKey(bookingPaymentIdempotencyKey(booking.ID))

	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, fmt.Errorf("create payment intent: %w", err)
	}

	return &PaymentResult{
		PaymentID:    pi.ID,
		Status:       string(pi.Status),
		Amount:       booking.TotalAmount,
		Currency:     s.config.Stripe.Currency,
		ClientSecret: pi.ClientSecret,
	}, nil
}

// PreparePaymentIntent creates an unconfirmed PaymentIntent to persist before charging.
func (s *PaymentService) PreparePaymentIntent(ctx context.Context, booking *models.Booking) (*PaymentResult, error) {
	if !s.isEnabled {
		return s.simulatePayment(booking)
	}

	params := s.newPaymentIntentParams(booking)
	params.SetIdempotencyKey(bookingPaymentIdempotencyKey(booking.ID))

	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, mapStripePaymentError(err)
	}

	return &PaymentResult{
		PaymentID:    pi.ID,
		Status:       string(pi.Status),
		Amount:       booking.TotalAmount,
		Currency:     s.config.Stripe.Currency,
		ClientSecret: pi.ClientSecret,
	}, nil
}

// ConfirmBookingPayment confirms a stored PaymentIntent, reusing it on purchase retries.
func (s *PaymentService) ConfirmBookingPayment(ctx context.Context, paymentIntentID string) (*PaymentResult, error) {
	if paymentIntentID == "" {
		return nil, ErrPaymentIntentMissing
	}

	if !s.isEnabled {
		return &PaymentResult{
			PaymentID: paymentIntentID,
			Status:    "succeeded",
			Currency:  "usd",
		}, nil
	}

	pi, err := paymentintent.Get(paymentIntentID, nil)
	if err != nil {
		return nil, fmt.Errorf("get payment intent: %w", err)
	}

	if pi.Status == stripe.PaymentIntentStatusSucceeded {
		return paymentResultFromIntent(pi), nil
	}

	confirmParams := &stripe.PaymentIntentConfirmParams{
		PaymentMethod: stripe.String("pm_card_visa"),
		ReturnURL:     stripe.String(s.config.Stripe.WebhookURL),
	}

	pi, err = paymentintent.Confirm(paymentIntentID, confirmParams)
	if err != nil {
		return nil, mapStripePaymentError(err)
	}

	if pi.Status != stripe.PaymentIntentStatusSucceeded {
		return nil, fmt.Errorf("%w: payment status is %s", ErrPaymentFailed, pi.Status)
	}

	return paymentResultFromIntent(pi), nil
}

// ProcessPayment confirms the booking's stored PaymentIntent.
func (s *PaymentService) ProcessPayment(ctx context.Context, booking *models.Booking) (*PaymentResult, error) {
	if booking.PaymentIntentID == nil || *booking.PaymentIntentID == "" {
		return nil, ErrPaymentIntentMissing
	}

	return s.ConfirmBookingPayment(ctx, *booking.PaymentIntentID)
}

// ConfirmPayment confirms a PaymentIntent after client-side authentication.
func (s *PaymentService) ConfirmPayment(ctx context.Context, paymentIntentID string) (*PaymentResult, error) {
	return s.ConfirmBookingPayment(ctx, paymentIntentID)
}

// RefundPayment processes a refund for a payment.
func (s *PaymentService) RefundPayment(ctx context.Context, paymentIntentID string, amount float64) error {
	if !s.isEnabled {
		return nil
	}

	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(paymentIntentID),
	}

	if amount > 0 {
		params.Amount = stripe.Int64(int64(amount * 100))
	}

	_, err := refund.New(params)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRefundFailed, err)
	}

	return nil
}

func (s *PaymentService) newPaymentIntentParams(booking *models.Booking) *stripe.PaymentIntentParams {
	amountInCents := int64(booking.TotalAmount * 100)

	return &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountInCents),
		Currency: stripe.String(s.config.Stripe.Currency),
		Metadata: map[string]string{
			"booking_id": booking.ID.String(),
			"event_id":   booking.EventID.String(),
			"user_id":    booking.UserID.String(),
			"seat_id":    booking.SeatID.String(),
		},
		Description: stripe.String(fmt.Sprintf("Event ticket booking: %s", booking.ID.String())),
	}
}

func bookingPaymentIdempotencyKey(bookingID uuid.UUID) string {
	return fmt.Sprintf("booking_%s", bookingID.String())
}

func paymentResultFromIntent(pi *stripe.PaymentIntent) *PaymentResult {
	return &PaymentResult{
		PaymentID: pi.ID,
		Status:    string(pi.Status),
		Amount:    float64(pi.Amount) / 100,
		Currency:  string(pi.Currency),
	}
}

func mapStripePaymentError(err error) error {
	stripeErr, ok := err.(*stripe.Error)
	if ok && stripeErr.Code == stripe.ErrorCodeCardDeclined {
		return ErrPaymentDeclined
	}
	return fmt.Errorf("%w: %v", ErrPaymentFailed, err)
}

// simulatePayment returns a simulated successful payment when Stripe is not configured.
func (s *PaymentService) simulatePayment(booking *models.Booking) (*PaymentResult, error) {
	return &PaymentResult{
		PaymentID: fmt.Sprintf("sim_%s", booking.ID.String()),
		Status:    "succeeded",
		Amount:    booking.TotalAmount,
		Currency:  "usd",
	}, nil
}
