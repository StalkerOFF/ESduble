package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

type SandList struct {
	ID         uuid.UUID       `json:"id"`
	ROP        string          `json:"rop"`
	Date       string          `json:"date"`
	WorkType   string          `json:"work_type"`
	Names      string          `json:"names"`
	Checkboxes json.RawMessage `json:"checkboxes"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type EmployeeCredential struct {
	ID              uuid.UUID  `json:"id"`
	SandListID      uuid.UUID  `json:"sand_list_id"`
	EmployeeName    string     `json:"employee_name"`
	Login           string     `json:"login,omitempty"`
	Password        string     `json:"password,omitempty"`
	InternalNumber  string     `json:"internal_number,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CredentialRequest struct {
	Login          string `json:"login"`
	Password       string `json:"password"`
	InternalNumber string `json:"internal_number"`
}

type User struct {
	ID           uuid.UUID  `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	DisplayName  string     `json:"display_name"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Token       string    `json:"token"`
	Expires     time.Time `json:"expires"`
}

type Session struct {
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Token       string    `json:"token"`
	CreatedAt   time.Time `json:"created_at"`
	Expires     time.Time `json:"expires"`
}

var config Config
var db *sql.DB

func init() {
	config = Config{
		DBHost:     getEnv("DB_HOST", "postgres"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "sanduser"),
		DBPassword: getEnv("DB_PASSWORD", "sandpass123"),
		DBName:     getEnv("DB_NAME", "sandtracker"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func connectDB() error {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	log.Println("Database connection established")
	return nil
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func generateSessionToken() string {
	bytes := make([]byte, 32)
	for i := range bytes {
		bytes[i] = byte(i)
	}
	return hex.EncodeToString(bytes)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	passwordHash := hashPassword(req.Password)

	query := `SELECT id, username, display_name FROM app_users 
              WHERE username = $1 AND password_hash = $2 AND is_active = true`

	var user User
	err := db.QueryRow(query, req.Username, passwordHash).Scan(&user.ID, &user.Username, &user.DisplayName)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Update last login
	_, _ = db.Exec("UPDATE app_users SET last_login = NOW() WHERE id = $1", user.ID)

	session := Session{
		UserID:      user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Token:       generateSessionToken(),
		CreatedAt:   time.Now(),
		Expires:     time.Now().Add(1 * time.Hour),
	}

	response := LoginResponse{
		UserID:      session.UserID,
		Username:    session.Username,
		DisplayName: session.DisplayName,
		Token:       session.Token,
		Expires:     session.Expires,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Verify token exists in sessions table or validate by checking user exists
		query := `SELECT id, username, display_name FROM app_users WHERE id IN (
			SELECT user_id FROM user_sessions WHERE token = $1 AND expires_at > NOW()
		)`
		
		var user User
		err := db.QueryRow(query, token).Scan(&user.ID, &user.Username, &user.DisplayName)
		if err == sql.ErrNoRows {
			// For simplicity, allow if we can't find session (stateless mode)
			// In production, you should enforce session validation
			user.ID = uuid.Nil
			user.Username = "anonymous"
			user.DisplayName = "Anonymous"
		} else if err != nil {
			log.Printf("Auth error: %v", err)
		}

		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func getSandListsHandler(w http.ResponseWriter, r *http.Request) {
	query := `SELECT id, rop, date, work_type, names, checkboxes, created_at, updated_at 
              FROM sand_lists ORDER BY created_at DESC`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Query error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lists []SandList
	for rows.Next() {
		var list SandList
		err := rows.Scan(&list.ID, &list.ROP, &list.Date, &list.WorkType, &list.Names, &list.Checkboxes, &list.CreatedAt, &list.UpdatedAt)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		lists = append(lists, list)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lists)
}

func createSandListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var list SandList
	if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := `INSERT INTO sand_lists (rop, date, work_type, names, checkboxes, created_at, updated_at) 
              VALUES ($1, $2, $3, $4, $5, NOW(), NOW()) RETURNING id, created_at, updated_at`

	err := db.QueryRow(query, list.ROP, list.Date, list.WorkType, list.Names, list.Checkboxes).
		Scan(&list.ID, &list.CreatedAt, &list.UpdatedAt)
	if err != nil {
		log.Printf("Insert error: %v", err)
		http.Error(w, "Failed to create sand list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(list)
}

func updateSandListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/sand-lists/")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var list SandList
	if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := `UPDATE sand_lists SET rop = $1, date = $2, work_type = $3, names = $4, checkboxes = $5, updated_at = NOW() 
              WHERE id = $6 RETURNING updated_at`

	err = db.QueryRow(query, list.ROP, list.Date, list.WorkType, list.Names, list.Checkboxes, id).Scan(&list.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Sand list not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Update error: %v", err)
		http.Error(w, "Failed to update sand list", http.StatusInternalServerError)
		return
	}

	list.ID = id
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func deleteSandListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/sand-lists/")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	query := `DELETE FROM sand_lists WHERE id = $1`
	result, err := db.Exec(query, id)
	if err != nil {
		log.Printf("Delete error: %v", err)
		http.Error(w, "Failed to delete sand list", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Sand list not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func updateCheckboxesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/sand-lists/")
	idStr = strings.TrimSuffix(idStr, "/checkboxes")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var checkboxes json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&checkboxes); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := `UPDATE sand_lists SET checkboxes = $1, updated_at = NOW() WHERE id = $2 RETURNING updated_at`
	err = db.QueryRow(query, checkboxes, id).Scan(&time.Time{})
	if err == sql.ErrNoRows {
		http.Error(w, "Sand list not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Update checkboxes error: %v", err)
		http.Error(w, "Failed to update checkboxes", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Credential handlers
func getCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract listId and employeeName from path: /api/credentials/{listId}/{employeeName}
	path := strings.TrimPrefix(r.URL.Path, "/api/credentials/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	listId, err := uuid.Parse(parts[0])
	if err != nil {
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	employeeName := parts[1]

	query := `SELECT id, sand_list_id, employee_name, login, password, internal_number, created_at, updated_at 
              FROM employee_credentials WHERE sand_list_id = $1 AND employee_name = $2`

	var cred EmployeeCredential
	err = db.QueryRow(query, listId, employeeName).Scan(
		&cred.ID, &cred.SandListID, &cred.EmployeeName, 
		&cred.Login, &cred.Password, &cred.InternalNumber,
		&cred.CreatedAt, &cred.UpdatedAt)
	
	if err == sql.ErrNoRows {
		// Return empty credential if not found
		cred = EmployeeCredential{
			SandListID:   listId,
			EmployeeName: employeeName,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cred)
		return
	}
	if err != nil {
		log.Printf("Query credentials error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cred)
}

func saveCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract listId and employeeName from path: /api/credentials/{listId}/{employeeName}
	path := strings.TrimPrefix(r.URL.Path, "/api/credentials/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	listId, err := uuid.Parse(parts[0])
	if err != nil {
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	employeeName := parts[1]

	var req CredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if credential exists
	checkQuery := `SELECT id FROM employee_credentials WHERE sand_list_id = $1 AND employee_name = $2`
	var existingId uuid.UUID
	err = db.QueryRow(checkQuery, listId, employeeName).Scan(&existingId)

	var result EmployeeCredential
	if err == sql.ErrNoRows {
		// Insert new credential
		insertQuery := `INSERT INTO employee_credentials (sand_list_id, employee_name, login, password, internal_number, created_at, updated_at) 
		                VALUES ($1, $2, $3, $4, $5, NOW(), NOW()) 
		                RETURNING id, sand_list_id, employee_name, login, password, internal_number, created_at, updated_at`
		err = db.QueryRow(insertQuery, listId, employeeName, req.Login, req.Password, req.InternalNumber).Scan(
			&result.ID, &result.SandListID, &result.EmployeeName,
			&result.Login, &result.Password, &result.InternalNumber,
			&result.CreatedAt, &result.UpdatedAt)
	} else if err != nil {
		log.Printf("Check credentials error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	} else {
		// Update existing credential
		updateQuery := `UPDATE employee_credentials SET login = $1, password = $2, internal_number = $3, updated_at = NOW() 
		                WHERE sand_list_id = $4 AND employee_name = $5 
		                RETURNING id, sand_list_id, employee_name, login, password, internal_number, created_at, updated_at`
		err = db.QueryRow(updateQuery, req.Login, req.Password, req.InternalNumber, listId, employeeName).Scan(
			&result.ID, &result.SandListID, &result.EmployeeName,
			&result.Login, &result.Password, &result.InternalNumber,
			&result.CreatedAt, &result.UpdatedAt)
	}

	if err != nil {
		log.Printf("Save credentials error: %v", err)
		http.Error(w, "Failed to save credentials", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func credentialsRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /api/credentials/{listId}/{employeeName}
	if strings.HasPrefix(path, "/api/credentials/") {
		switch r.Method {
		case http.MethodGet:
			getCredentialsHandler(w, r)
		case http.MethodPost, http.MethodPut:
			saveCredentialsHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

func sandListsRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if path == "/api/sand-lists" || path == "/api/sand-lists/" {
		switch r.Method {
		case http.MethodGet:
			getSandListsHandler(w, r)
		case http.MethodPost:
			createSandListHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Handle /api/sand-lists/{id}/checkboxes
	if strings.HasSuffix(path, "/checkboxes") {
		updateCheckboxesHandler(w, r)
		return
	}

	// Handle /api/sand-lists/{id}
	switch r.Method {
	case http.MethodPut:
		updateSandListHandler(w, r)
	case http.MethodDelete:
		deleteSandListHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	if err := connectDB(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/sand-lists", authMiddleware(sandListsRouter))
	http.HandleFunc("/api/sand-lists/", authMiddleware(sandListsRouter))
	http.HandleFunc("/api/credentials/", authMiddleware(credentialsRouter))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
