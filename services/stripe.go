package services

import (
	"fmt"
	"os"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/customer"
)

var useStripe bool

func InitStripe() {
	useStripe = os.Getenv("USE_STRIPE") == "true"
	if useStripe {
		stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
		fmt.Println("Stripe integration ON")
	} else {
		fmt.Println("Stripe integration OFF")
	}
}

func CreateCustomer() (string, error) {
	if !useStripe {
		return "mock_customer_id", nil
	}

	params := &stripe.CustomerParams{}
	cust, err := customer.New(params)
	if err != nil {
		return "", err
	}
	return cust.ID, nil
}

func ChargeCustomer(amount int64, currency, source, description string) (*stripe.Charge, error) {
	if !useStripe {
		return &stripe.Charge{
			ID:     "mock_charge_id",
			Amount: amount,
			Status: "succeeded",
		}, nil
	}

	params := &stripe.ChargeParams{
		Amount:      stripe.Int64(amount),
		Currency:    stripe.String(currency),
		Description: stripe.String(description),
	}
	params.SetSource(source)

	return charge.New(params)
}
