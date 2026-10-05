package service

import "example.com/app/internal/store"

// Run reads from the store.
func Run() string { return store.Get() }
