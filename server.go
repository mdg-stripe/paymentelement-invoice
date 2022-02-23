package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/invoice"
	"github.com/stripe/stripe-go/v72/invoiceitem"
	"github.com/stripe/stripe-go/v72/paymentmethod"
	"github.com/stripe/stripe-go/v72/setupintent"
)

func main() {
	// This is a public sample test API key.
	// Don’t submit any personally identifiable information in requests made with this key.
	// Sign in to see your own test API key embedded in code samples.
	stripe.Key = "sk_test_..."

	fs := http.FileServer(http.Dir("public"))
	http.Handle("/", fs)
	http.HandleFunc("/create-setup-intent", handleCreateSetupIntent)
	http.HandleFunc("/confirm", confirmHandler)

	addr := "localhost:4242"
	log.Printf("Listening on %s ...", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleCreateSetupIntent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Create a SetupIntent
	// STRIPE DOCS: https://stripe.com/docs/api/setup_intents/create
	params := &stripe.SetupIntentParams{Usage: stripe.String("off_session")}
	si, err := setupintent.New(params)
	log.Printf("si.New: %v", si.ClientSecret)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("si.New: %v", err)
		return
	}

	writeJSON(w, struct {
		ClientSecret string `json:"clientSecret"`
	}{
		ClientSecret: si.ClientSecret,
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("json.NewEncoder.Encode: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, &buf); err != nil {
		log.Printf("io.Copy: %v", err)
		return
	}
}

func confirmHandler(w http.ResponseWriter, r *http.Request) {
	// get the setup intent ID
	setupIntentID := r.URL.Query().Get("setup_intent")
	if setupIntentID == "" {
		log.Fatalf("ERROR: setupIntentID is empty.")
	}
	log.Printf("Setup intent ID is: %s", setupIntentID)

	// STRIPE DOCS: https://stripe.com/docs/api/setup_intents/retrieve
	si, err := setupintent.Get(setupIntentID, nil)
	if err != nil {
		log.Fatal(err)
	}

	// attach the payment method to the customer
	customerID := "cus_..."
	// STRIPE DOCS: https://stripe.com/docs/api/payment_methods/attach
	attachParams := &stripe.PaymentMethodAttachParams{Customer: stripe.String(customerID)}
	pm, err := paymentmethod.Attach(si.PaymentMethod.ID, attachParams)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("PaymentMethod: %s successfully attached to customer: %s", pm.ID, customerID)

	// create an invoice with line items
	// STRIPE DOCS: https://stripe.com/docs/api/invoiceitems/create
	lineItemParams := &stripe.InvoiceItemParams{Customer: stripe.String(customerID),
		Price: stripe.String("price_...")}
	_, err = invoiceitem.New(lineItemParams)
	if err != nil {
		log.Fatal(err)
	}

	// STRIPE DOCS: https://stripe.com/docs/api/invoices/create
	invoiceParams := &stripe.InvoiceParams{Customer: stripe.String(customerID),
		CollectionMethod: stripe.String("charge_automatically")}
	inv, err := invoice.New(invoiceParams)
	if err != nil {
		log.Fatal(err)
	}

	// STRIPE DOCS: https://stripe.com/docs/api/invoices/finalize
	inv, err = invoice.FinalizeInvoice(inv.ID, nil)
	if err != nil {
		log.Fatal(err)
	}

	// STRIPE DOCS: https://stripe.com/docs/api/invoices/pay
	inv, err = invoice.Pay(inv.ID, &stripe.InvoicePayParams{PaymentMethod: &si.PaymentMethod.ID})
	if err != nil {
		log.Fatal(err)
	}

	w.Write([]byte(fmt.Sprintf("Successfully paid invoice: %s and confirmed payment intent: %s", inv.ID, inv.PaymentIntent.ID)))
}
