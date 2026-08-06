package db

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	log "github.com/shyunku-libraries/go-logger"
	"os"
)

var Root *Database = nil

type Database struct {
	*sqlx.DB
}

type DatabaseProcess struct {
	Id      string  `db:"Id"`
	User    *string `db:"User"`
	Host    *string `db:"Host"`
	Db      *string `db:"db"`
	Command *string `db:"Command"`
	Time    *string `db:"Time"`
	State   *string `db:"State"`
	Info    *string `db:"Info"`
}

func (d *Database) Finalize() error {
	// get all process
	var processList []DatabaseProcess
	if err := d.Select(&processList, "SHOW PROCESSLIST"); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	for _, process := range processList {
		if process.Db != nil && *process.Db == "teamgg" {
			_, _ = d.Exec(fmt.Sprintf("KILL %s", process.Id))
			query := "???"
			if process.Info != nil {
				query = *process.Info
			}
			log.Debugf("Killed process: %s --- %s", process.Id, query)
		}
	}
	return d.Close()
}

type DatabaseInitializer func(db *sqlx.DB) error

func getEndpoint() string {
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	databaseName := os.Getenv("DB_NAME")

	databaseConfig := NewDatabaseConfig(user, password, host, port, databaseName)
	endpoint := databaseConfig.getEndpointString()

	return endpoint
}

type DatabaseConfig struct {
	User         string
	Password     string
	Host         string
	Port         string
	DatabaseName string
}

func NewDatabaseConfig(user, pass, host, port, databaseName string) *DatabaseConfig {
	return &DatabaseConfig{
		User:         user,
		Password:     pass,
		Host:         host,
		Port:         port,
		DatabaseName: databaseName,
	}
}

func (d *DatabaseConfig) validate() error {
	if d.User == "" {
		return errors.New("user is required")
	}
	if d.Password == "" {
		return errors.New("password is required")
	}
	if d.Host == "" {
		return errors.New("host is required")
	}
	if d.Port == "" {
		return errors.New("port is required")
	}
	if d.DatabaseName == "" {
		return errors.New("database name is required")
	}
	return nil
}

func (d *DatabaseConfig) getEndpointString() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", d.User, d.Password, d.Host, d.Port, d.DatabaseName)
}

func Initiate(initializer DatabaseInitializer) (*Database, error) {
	endPoint := getEndpoint()
	db, err := sqlx.Open("mysql", endPoint)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, errors.New("failed to connect database: " + err.Error())
	}
	if initializer != nil {
		if err := initializer(db); err != nil {
			return nil, errors.New("failed to initialize database: " + err.Error())
		}
	}
	db.SetConnMaxIdleTime(0)
	db.SetMaxIdleConns(100)
	db.SetMaxOpenConns(100)
	return &Database{
		DB: db,
	}, nil
}

type Context interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Get(dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
	Rebind(query string) string
}

type TxContext interface {
	Context
	Commit() error
	Rollback() error
}
