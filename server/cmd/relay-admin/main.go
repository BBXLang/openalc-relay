// relay-admin performs local-only initialization; it exposes no HTTP routes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/secrets"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: relay-admin keygen|bootstrap [options]")
	}
	switch args[0] {
	case "keygen":
		flags := flag.NewFlagSet("keygen", flag.ContinueOnError)
		path := flags.String("out", "", "new key file path")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *path == "" {
			return errors.New("usage: relay-admin keygen -out PATH")
		}
		if err := secrets.CreateFile(*path); err != nil {
			return err
		}
		fmt.Println("encryption key file created")
		return nil
	case "bootstrap":
		flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
		database := flags.String("database", "", "new or existing Relay database")
		keyFile := flags.String("key-file", "", "encryption key file")
		passwordFile := flags.String("password-file", "", "owner password file")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 ||
			*database == "" || *keyFile == "" || *passwordFile == "" {
			return errors.New("usage: relay-admin bootstrap -database PATH -key-file PATH -password-file PATH")
		}
		if _, err := secrets.LoadFile(*keyFile); err != nil {
			return fmt.Errorf("encryption keys invalid: %w", err)
		}
		password, err := readPassword(*passwordFile)
		if err != nil {
			return err
		}
		defer func() {
			for i := range password {
				password[i] = 0
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		db, err := sqlite.Open(ctx, *database)
		if err != nil {
			return errors.New("Relay database unavailable or migration failed")
		}
		defer db.Close()
		if err := identity.Bootstrap(ctx, db, password); err != nil {
			return err
		}
		fmt.Println("owner initialized")
		return nil
	default:
		return errors.New("usage: relay-admin keygen|bootstrap [options]")
	}
}

func readPassword(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1026 {
		return nil, errors.New("password file unavailable or invalid")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("password file permissions must be owner-only")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("password file unreadable")
	}
	// A text editor may add one line ending; spaces remain part of the password.
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
		if len(data) > 0 && data[len(data)-1] == '\r' {
			data = data[:len(data)-1]
		}
	}
	if len(data) < 12 || len(data) > 1024 {
		return nil, identity.ErrInvalidPassword
	}
	return data, nil
}
