package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"golang.org/x/term"
)

const minPasswordLen = 8

func runUser(ctx context.Context, conn *db.DB, sub string, args []string) int {
	switch sub {
	case "add":
		return userAdd(ctx, conn, args)
	case "list":
		return userList(ctx, conn)
	case "delete":
		return userDelete(ctx, conn, args)
	case "password":
		return userPassword(ctx, conn, args)
	default:
		return errf("user: unknown subcommand %q", sub)
	}
}

func userAdd(ctx context.Context, conn *db.DB, args []string) int {
	fs := flag.NewFlagSet("user add", flag.ContinueOnError)
	email := fs.String("email", "", "email address (required)")
	name := fs.String("name", "", "display name (required)")
	base := fs.String("base-currency", "EUR", "reporting currency")
	pwFlag := fs.String("password", "",
		"password (avoid in shell history; prefer the interactive prompt)")
	skipPw := fs.Bool("no-password", false,
		"create the user without a password (API-token-only access)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *email == "" || *name == "" {
		return errf("user add: --email and --name are required")
	}
	cur, err := domain.ParseCurrency(*base)
	if err != nil {
		return errf("user add: %v", err)
	}

	var hash string
	switch {
	case *skipPw:
		hash = ""
	case *pwFlag != "":
		if len(*pwFlag) < minPasswordLen {
			return errf("user add: password must be at least %d characters", minPasswordLen)
		}
		hash, err = auth.HashPassword(*pwFlag)
		if err != nil {
			return errf("user add: hash: %v", err)
		}
	default:
		pw, err := readPasswordTwice("Password: ", "Confirm: ")
		if err != nil {
			return errf("user add: %v", err)
		}
		hash, err = auth.HashPassword(pw)
		if err != nil {
			return errf("user add: hash: %v", err)
		}
	}

	u := &domain.User{
		Email: *email, Name: *name,
		PasswordHash: hash, BaseCurrency: cur,
	}
	if err := conn.CreateUser(ctx, u); err != nil {
		return errf("user add: %v", err)
	}
	fmt.Printf("user created: id=%d email=%s\n", u.ID, u.Email)
	return 0
}

func userPassword(ctx context.Context, conn *db.DB, args []string) int {
	fs := flag.NewFlagSet("user password", flag.ContinueOnError)
	email := fs.String("email", "", "email address (required)")
	pwFlag := fs.String("password", "",
		"new password (avoid in shell history; prefer the interactive prompt)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *email == "" {
		return errf("user password: --email is required")
	}
	u, err := conn.GetUserByEmail(ctx, *email)
	if errors.Is(err, db.ErrNotFound) {
		return errf("user password: no such email %q", *email)
	}
	if err != nil {
		return errf("user password: %v", err)
	}

	var pw string
	if *pwFlag != "" {
		pw = *pwFlag
	} else {
		pw, err = readPasswordTwice("New password: ", "Confirm: ")
		if err != nil {
			return errf("user password: %v", err)
		}
	}
	if len(pw) < minPasswordLen {
		return errf("user password: must be at least %d characters", minPasswordLen)
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return errf("user password: hash: %v", err)
	}
	if err := conn.UpdateUserPassword(ctx, u.ID, hash); err != nil {
		return errf("user password: %v", err)
	}
	// Kill every browser session for this user — they need to log back in.
	_ = conn.DeleteUserSessionsExcept(ctx, u.ID, "")
	fmt.Printf("password updated for %s\n", u.Email)
	return 0
}

func userList(ctx context.Context, conn *db.DB) int {
	users, err := conn.ListUsers(ctx)
	if err != nil {
		return errf("user list: %v", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tEMAIL\tNAME\tBASE\tPASSWORD\tCREATED")
	for _, u := range users {
		pwStatus := "—"
		if u.PasswordHash != "" {
			pwStatus = "set"
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			u.ID, u.Email, u.Name, u.BaseCurrency, pwStatus,
			u.CreatedAt.Format("2006-01-02"))
	}
	_ = w.Flush()
	return 0
}

func userDelete(ctx context.Context, conn *db.DB, args []string) int {
	fs := flag.NewFlagSet("user delete", flag.ContinueOnError)
	id := fs.Int64("id", 0, "user id")
	email := fs.String("email", "", "email address")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *id == 0 && *email == "" {
		return errf("user delete: --id or --email required")
	}
	if *id == 0 {
		u, err := conn.GetUserByEmail(ctx, *email)
		if errors.Is(err, db.ErrNotFound) {
			return errf("user delete: no such email %q", *email)
		}
		if err != nil {
			return errf("user delete: %v", err)
		}
		*id = u.ID
	}
	if err := conn.DeleteUser(ctx, *id); err != nil {
		return errf("user delete: %v", err)
	}
	fmt.Printf("user deleted: id=%d\n", *id)
	return 0
}

// readPasswordTwice prompts for a password with terminal echo off, then
// asks for confirmation. Falls back to plain stdin read when stdin is
// not a terminal (e.g., piped). Enforces minPasswordLen.
func readPasswordTwice(prompt1, prompt2 string) (string, error) {
	p1, err := readPassword(prompt1)
	if err != nil {
		return "", err
	}
	if len(p1) < minPasswordLen {
		return "", fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	p2, err := readPassword(prompt2)
	if err != nil {
		return "", err
	}
	if p1 != p2 {
		return "", errors.New("passwords do not match")
	}
	return p1, nil
}

func readPassword(prompt string) (string, error) {
	fd := int(os.Stdin.Fd()) //nolint:gosec // fd is always a small non-negative uintptr
	if term.IsTerminal(fd) {
		fmt.Print(prompt)
		b, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	// Piped stdin: read a single line.
	s := bufio.NewScanner(os.Stdin)
	if !s.Scan() {
		if err := s.Err(); err != nil {
			return "", err
		}
		return "", errors.New("no password on stdin")
	}
	return strings.TrimRight(s.Text(), "\r\n"), nil
}
