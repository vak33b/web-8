package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"

	_ "github.com/lib/pq" // Импорт драйвера для PostgreSQL
)

// Структура для параметров подключения к базе данных
type dbConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string // Добавлено для безопасности
}

// Структура для хранения приветственных сообщений
type Greeting struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}

// DatabaseProvider обрабатывает взаимодействие с базой данных
type DatabaseProvider struct {
	db *sql.DB
}

// NewDatabaseProvider создает новый экземпляр DatabaseProvider
func NewDatabaseProvider(cfg dbConfig) (*DatabaseProvider, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("не удалось проверить соединение с базой данных: %w", err)
	}

	return &DatabaseProvider{db: db}, nil
}

// GreetUser приветствует пользователя и сохраняет приветствие в базе данных.
func (dp *DatabaseProvider) GreetUser(name string) (*Greeting, error) {
	// Подготавливаем SQL-запрос для предотвращения SQL-инъекций.
	stmt, err := dp.db.Prepare("INSERT INTO greetings (message) VALUES ($1) RETURNING id, message")
	if err != nil {
		return nil, fmt.Errorf("не удалось подготовить запрос: %w", err)
	}
	defer stmt.Close() // Важно: Закрываем запрос после использования.

	var greeting Greeting
	err = stmt.QueryRow(fmt.Sprintf("Hello, %s!", name)).Scan(&greeting.ID, &greeting.Message)
	if err != nil {
		return nil, fmt.Errorf("не удалось вставить приветствие: %w", err)
	}

	return &greeting, nil
}

// Функция для получения всех приветствий из базы данных
func (dp *DatabaseProvider) GetAllGreetings() ([]Greeting, error) {
	rows, err := dp.db.Query("SELECT id, message FROM greetings")
	if err != nil {
		return nil, fmt.Errorf("failed to query greetings: %w", err)
	}
	defer rows.Close()

	var greetings []Greeting
	for rows.Next() {
		var greeting Greeting
		err := rows.Scan(&greeting.ID, &greeting.Message)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		greetings = append(greetings, greeting)
	}

	return greetings, nil
}

// Новый обработчик для получения всех приветствий
func getAllGreetingsHandler(dbProvider *DatabaseProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		greetings, err := dbProvider.GetAllGreetings()
		if err != nil {
			http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(greetings)
	}
}

// Функция обработчика HTTP-запросов
func greetHandler(dbProvider *DatabaseProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			http.Error(w, "Неверные параметры запроса", http.StatusBadRequest)
			return
		}

		name := query.Get("name")
		if name == "" {
			http.Error(w, "Отсутствует параметр 'name'", http.StatusBadRequest)
			return
		}

		greeting, err := dbProvider.GreetUser(name)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка базы данных: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(greeting)
	}
}

func main() {
	// Настройка с помощью флагов
	var dbConfig dbConfig
	flag.StringVar(&dbConfig.Host, "db-host", "localhost", "Хост базы данных")
	flag.IntVar(&dbConfig.Port, "db-port", 5432, "Порт базы данных")
	flag.StringVar(&dbConfig.User, "db-user", "postgres", "Пользователь базы данных")
	flag.StringVar(&dbConfig.Password, "db-password", "sana2005A", "Пароль базы данных")
	flag.StringVar(&dbConfig.DBName, "db-dbname", "sandbox", "Имя базы данных")
	flag.StringVar(&dbConfig.SSLMode, "db-sslmode", "disable", "Режим SSL для базы данных (disable, require, verify-ca, verify-full)")
	flag.Parse()

	// Создаем провайдер для базы данных
	dbProvider, err := NewDatabaseProvider(dbConfig)
	if err != nil {
		log.Fatalf("Ошибка при создании провайдера базы данных: %v", err)
	}
	defer dbProvider.db.Close()

	http.HandleFunc("/api/user", greetHandler(dbProvider))
	http.HandleFunc("/api/greetings", getAllGreetingsHandler(dbProvider))
	fmt.Println("Сервер запущен на порту :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
