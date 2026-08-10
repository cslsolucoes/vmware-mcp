//go:build !windows

package main

// Platform só compila neste binário quando GOOS != windows (linux, darwin, etc.).
func Platform() string {
	return "unix"
}
