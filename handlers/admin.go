package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// POST /api/admin/products
func CreateProduct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Price       int    `json:"price"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "Invalid input", http.StatusBadRequest)
			return
		}

		_, err := db.Exec(`INSERT INTO products (name, description, price) VALUES ($1, $2, $3)`, p.Name, p.Description, p.Price)
		if err != nil {
			http.Error(w, "Error creating product", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "Product created"})
	}
}

// PUT /api/admin/products/{id}
func UpdateProduct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]

		var p struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Price       int    `json:"price"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "Invalid input", http.StatusBadRequest)
			return
		}

		_, err := db.Exec(`UPDATE products SET name=$1, description=$2, price=$3 WHERE id=$4`, p.Name, p.Description, p.Price, id)
		if err != nil {
			http.Error(w, "Error updating product", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Product updated"})
	}
}

// DELETE /api/admin/products/{id}
func DeleteProduct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]

		_, err := db.Exec(`UPDATE products SET deleted = TRUE WHERE id = $1`, id)

		if err != nil {
			http.Error(w, "Error deleting product", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Product deleted"})
	}
}

// GET /api/admin/sales
func GetSalesReport(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := `
            SELECT u.name, p.name, p.price, pur.purchase_date
            FROM purchases pur
            JOIN users u ON pur.user_id = u.id
            JOIN products p ON pur.product_id = p.id
            WHERE ($1::text IS NULL OR u.name ILIKE '%' || $1 || '%')
              AND ($2::timestamp IS NULL OR pur.purchase_date >= $2)
              AND ($3::timestamp IS NULL OR pur.purchase_date <= $3)
        `

		user := r.URL.Query().Get("user")
		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")

		var rows *sql.Rows
		var err error
		if rows, err = db.Query(query, nullOrString(user), nullOrTime(from), nullOrTime(to)); err != nil {
			http.Error(w, "Query failed", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var results []map[string]interface{}
		for rows.Next() {
			var uname, pname string
			var price int
			var date time.Time
			rows.Scan(&uname, &pname, &price, &date)
			results = append(results, map[string]interface{}{
				"user":    uname,
				"product": pname,
				"price":   price,
				"date":    date,
			})
		}

		json.NewEncoder(w).Encode(results)
	}
}

func nullOrString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullOrTime(s string) interface{} {
	if s == "" {
		return nil
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
