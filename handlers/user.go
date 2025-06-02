package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// POST /signup
func SignUp(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Email == "" || req.Password == "" || req.Name == "" {
			http.Error(w, "Missing fields", http.StatusBadRequest)
			return
		}

		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Error hashing password", http.StatusInternalServerError)
			return
		}

		// Insert into DB
		_, err = db.Exec(`INSERT INTO users (name, email, password, role) VALUES ($1, $2, $3, $4)`, req.Name, req.Email, string(hash), req.Role)
		if err != nil {
			fmt.Println("Database Insert Error:", err) // Logs error to console
			http.Error(w, fmt.Sprintf("Error creating user: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "User created"})
	}
}

// POST /login
func Login(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		var userID int
		var hashedPassword string
		var role string
		err := db.QueryRow(`SELECT id, password, role FROM users WHERE email = $1`, req.Email).Scan(&userID, &hashedPassword, &role)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": userID,
			"role":    role,
			"exp":     time.Now().Add(72 * time.Hour).Unix(),
		})

		tokenStr, err := token.SignedString([]byte("your-secret-key"))
		if err != nil {
			fmt.Println("Database Insert Error:", err) // Logs error to console
			http.Error(w, "Token generation failed", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"token": tokenStr})
	}
}

// POST /api/credit-card
func AddCreditCard(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value("user_id").(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			CardNumber string `json:"card_number"`
			ExpMonth   int    `json:"exp_month"`
			ExpYear    int    `json:"exp_year"`
			CVC        string `json:"cvc"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.CardNumber == "" || req.CVC == "" || req.ExpMonth == 0 || req.ExpYear == 0 {
			http.Error(w, "Missing card fields", http.StatusBadRequest)
			return
		}

		// Insert the credit card and return its ID
		var creditCardID int
		err := db.QueryRow(`
            INSERT INTO credit_cards (card_number, exp_month, exp_year, cvc)
            VALUES ($1, $2, $3, $4) RETURNING id
        `, req.CardNumber, req.ExpMonth, req.ExpYear, req.CVC).Scan(&creditCardID)

		if err != nil {
			fmt.Println("Database Insert Error:", err)
			http.Error(w, "Could not save card", http.StatusInternalServerError)
			return
		}

		// Update the user to associate the credit card
		_, err = db.Exec(`
            UPDATE users SET credit_card_id = $1 WHERE id = $2
        `, creditCardID, userID)

		if err != nil {
			fmt.Println("User Update Error:", err)
			http.Error(w, "Could not link credit card to user", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "Card saved and linked to user"})
	}
}

// DELETE /api/credit-card
func DeleteCreditCard(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value("user_id").(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "Transaction start failed", http.StatusInternalServerError)
			return
		}

		// Step 1: get credit_card_id
		var cardID int
		err = tx.QueryRow(`SELECT credit_card_id FROM users WHERE id = $1`, userID).Scan(&cardID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to fetch card info", http.StatusInternalServerError)
			return
		}

		if cardID == 0 {
			tx.Rollback()
			http.Error(w, "No card on file", http.StatusBadRequest)
			return
		}

		// Step 2: unlink card from user
		_, err = tx.Exec(`UPDATE users SET credit_card_id = NULL WHERE id = $1`, userID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to unlink card", http.StatusInternalServerError)
			return
		}

		// Step 3: delete card from credit_cards
		_, err = tx.Exec(`DELETE FROM credit_cards WHERE id = $1`, cardID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Failed to delete card", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Commit failed", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Credit card unlinked and deleted",
		})
	}
}

// GET /api/products
func ListProducts(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT id, name, description, price 
			FROM products 
			WHERE deleted = FALSE
		`)
		if err != nil {
			http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type Product struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Price       int    `json:"price"`
		}

		var products []Product
		for rows.Next() {
			var p Product
			if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price); err != nil {
				http.Error(w, "Error reading product", http.StatusInternalServerError)
				return
			}
			products = append(products, p)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(products)
	}
}

// POST /api/purchase
func PurchaseProducts(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int)

		var req struct {
			ProductIDs []int `json:"product_ids"`
			//Source     string `json:"source"` // e.g., credit card token
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if len(req.ProductIDs) == 0 {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		// Fetch total amount
		rows, err := db.Query(`
			SELECT id, price 
			FROM products 
			WHERE id = ANY($1) AND deleted = FALSE
		`, pq.Array(req.ProductIDs))

		if err != nil {
			http.Error(w, "Failed to fetch product prices", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var total int64
		for rows.Next() {
			var price int
			rows.Scan(&price)
			total += int64(price)
		}

		// Stripe charge
		/*
			charge, err := services.ChargeCustomer(total, "usd", req.Source, fmt.Sprintf("Purchase by user %d", userID))
			if err != nil {
				http.Error(w, "Payment failed", http.StatusPaymentRequired)
				return
			}
		*/

		// Insert purchases

		tx, _ := db.Begin()
		stmt, _ := tx.Prepare(`INSERT INTO purchases (user_id, product_id, purchase_date) VALUES ($1, $2, $3)`)
		for _, pid := range req.ProductIDs {
			stmt.Exec(userID, pid, time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC))
		}
		stmt.Close()
		tx.Commit()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Purchase successful",
		})
	}
}

// GET /api/purchases
func GetUserPurchases(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value("user_id").(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		query := `
            SELECT p.id, p.name, p.description, p.price, pu.purchase_date
            FROM purchases pu
            JOIN products p ON pu.product_id = p.id
            WHERE pu.user_id = $1
            ORDER BY pu.purchase_date DESC
        `

		rows, err := db.Query(query, userID)
		if err != nil {
			http.Error(w, "Failed to fetch purchases", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type Purchase struct {
			ProductID    int       `json:"product_id"`
			Name         string    `json:"name"`
			Description  string    `json:"description"`
			Price        int       `json:"price"`
			PurchaseDate time.Time `json:"purchase_date"`
		}

		var purchases []Purchase
		for rows.Next() {
			var p Purchase
			if err := rows.Scan(&p.ProductID, &p.Name, &p.Description, &p.Price, &p.PurchaseDate); err != nil {
				http.Error(w, "Error scanning purchase", http.StatusInternalServerError)
				return
			}
			purchases = append(purchases, p)
		}

		json.NewEncoder(w).Encode(purchases)
	}
}
