package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"github.com/mstoups/server-backend/handlers"
	"github.com/mstoups/server-backend/middleware"
	"github.com/mstoups/server-backend/services"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@db:5432/app_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer db.Close()

	// Initialize Stripe
	services.InitStripe()

	// Set up router
	r := mux.NewRouter()

	// Public routes
	r.HandleFunc("/signup", handlers.SignUp(db)).Methods("POST")
	r.HandleFunc("/login", handlers.Login(db)).Methods("POST")

	// Auth routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(middleware.AuthMiddleware)

	// User endpoints
	api.HandleFunc("/credit-card", handlers.AddCreditCard(db)).Methods("POST")
	api.HandleFunc("/credit-card", handlers.DeleteCreditCard(db)).Methods("DELETE")
	api.HandleFunc("/products", handlers.ListProducts(db)).Methods("GET")
	api.HandleFunc("/purchase", handlers.PurchaseProducts(db)).Methods("POST")
	api.HandleFunc("/purchases", handlers.GetUserPurchases(db)).Methods("GET")

	// Admin endpoints
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(middleware.AdminMiddleware)
	admin.HandleFunc("/products", handlers.CreateProduct(db)).Methods("POST")
	admin.HandleFunc("/products/{id}", handlers.UpdateProduct(db)).Methods("PUT")
	admin.HandleFunc("/products/{id}", handlers.DeleteProduct(db)).Methods("DELETE")
	admin.HandleFunc("/sales", handlers.GetSalesReport(db)).Methods("GET")

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Listening on port %s 🚀", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
