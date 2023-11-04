package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	_ "github.com/lib/pq"
)

type Todo struct {
	ID          int          `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Completed   bool         `json:"completed"`
	CompletedAt sql.NullTime `json:"completed_at,omitempty"`
}

func newTodo(id int, title string, description string) *Todo {
	return &Todo{
		ID:          id,
		Title:       title,
		Description: description,
		Completed:   false,
		CompletedAt: sql.NullTime{Valid: false},
	}
}

const createTableQuery = `CREATE TABLE IF NOT EXISTS todo (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMP WITH TIME ZONE
);`

func createTable(db *sql.DB) error {
	_, err := db.Exec(createTableQuery)
	return err
}

func connectToDB() *sql.DB {
	connStr := "user=admin password=admin dbname=go-todo sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	err = createTable(db)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func main() {
	http.HandleFunc("/api/todo", todoHandler)
	http.ListenAndServe(":8080", nil)
}

func todoHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			getTodoHandler(w, r)
		} else {
			getTodoList(w, r)
		}
	case http.MethodPost:
		createTodoHandler(w, r)
	case http.MethodPut:
		updateTodoHandler(w, r)
	case http.MethodDelete:
		deleteTodoHandler(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func getTodoList(w http.ResponseWriter, r *http.Request) {
	db := connectToDB()
	defer db.Close()

	var todoList []Todo

	rows, err := db.Query("SELECT id, title, description, completed, completed_at FROM todo")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var todo Todo
		err := rows.Scan(&todo.ID, &todo.Title, &todo.Description, &todo.Completed, &todo.CompletedAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		todoList = append(todoList, todo)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(todoList)
}

func getTodoHandler(w http.ResponseWriter, r *http.Request) {
	db := connectToDB()
	defer db.Close()

	queryParams := r.URL.Query()
	id, err := strconv.Atoi(queryParams.Get("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var todo Todo

	sqlStatement := `SELECT id, title, description, completed, completed_at FROM todo WHERE id = $1`
	row := db.QueryRow(sqlStatement, id)
	err = row.Scan(&todo.ID, &todo.Title, &todo.Description, &todo.Completed, &todo.CompletedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(todo)
}

func createTodoHandler(w http.ResponseWriter, r *http.Request) {
	db := connectToDB()
	defer db.Close()

	var todo Todo

	err := json.NewDecoder(r.Body).Decode(&todo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlStatement := `
	INSERT INTO todo (title, description, completed, completed_at)
	VALUES ($1, $2, $3, $4) RETURNING id`

	var id int
	err = db.QueryRow(sqlStatement, todo.Title, todo.Description, todo.Completed, todo.CompletedAt).Scan(&id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	todo.ID = id

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(todo)
}

func updateTodoHandler(w http.ResponseWriter, r *http.Request) {
	db := connectToDB()
	defer db.Close()

	var todo Todo

	err := json.NewDecoder(r.Body).Decode(&todo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	queryParams := r.URL.Query()
	id, err := strconv.Atoi(queryParams.Get("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	todo.ID = id

	sqlStatement := `
	UPDATE todo
	SET title = $2, description = $3, completed = $4
	WHERE id = $1`

	res, err := db.Exec(sqlStatement, todo.ID, todo.Title, todo.Description, todo.Completed)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// check how many rows were affected
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Todo not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(todo)
}

func deleteTodoHandler(w http.ResponseWriter, r *http.Request) {
	db := connectToDB()
	defer db.Close()

	queryParams := r.URL.Query()
	id, err := strconv.Atoi(queryParams.Get("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlStatement := `DELETE FROM todo WHERE id = $1`
	res, err := db.Exec(sqlStatement, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Todo not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}
