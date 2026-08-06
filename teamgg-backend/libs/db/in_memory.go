package db

import (
	"errors"
	"time"
)

var InMemoryDB InMemoryDatabase

var (
	ErrValueNotFound = errors.New("value not found")
)

type InMemoryDatabase interface {
	Set(key string, value string) error
	SetExp(key string, value string, expires time.Duration) error
	Get(key string) (string, error)
	Del(key string) error
	LPush(key string, value string) error
	LPushExp(key string, value string, expires time.Duration) error
	LRange(key string, start int64, stop int64) ([]string, error)
	RPush(key string, value string) error
	RPushExp(key string, value string, expires time.Duration) error
	LLen(key string) (int64, error)
	LRem(key string, count int64, value string) error
	Expire(key string, expiration time.Duration) error
}
