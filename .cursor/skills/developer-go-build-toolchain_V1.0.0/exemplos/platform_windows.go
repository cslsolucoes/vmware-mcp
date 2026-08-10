//go:build windows

package main

// Platform só compila neste binário quando GOOS=windows.
func Platform() string {
	return "windows"
}
