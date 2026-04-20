package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func runUser(ctx context.Context, conn *db.DB, sub string, args []string) int {
	switch sub {
	case "add":
		return userAdd(ctx, conn, args)
	case "list":
		return userList(ctx, conn)
	case "delete":
		return userDelete(ctx, conn, args)
	default:
		return errf("user: unknown subcommand %q", sub)
	}
}

func userAdd(ctx context.Context, conn *db.DB, args []string) int {
	fs := flag.NewFlagSet("user add", flag.ContinueOnError)
	email := fs.String("email", "", "email address (required)")
	name := fs.String("name", "", "display name (required)")
	base := fs.String("base-currency", "EUR", "reporting currency")
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
	u := &domain.User{Email: *email, Name: *name, BaseCurrency: cur}
	if err := conn.CreateUser(ctx, u); err != nil {
		return errf("user add: %v", err)
	}
	fmt.Printf("user created: id=%d email=%s\n", u.ID, u.Email)
	return 0
}

func userList(ctx context.Context, conn *db.DB) int {
	users, err := conn.ListUsers(ctx)
	if err != nil {
		return errf("user list: %v", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tEMAIL\tNAME\tBASE\tCREATED")
	for _, u := range users {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			u.ID, u.Email, u.Name, u.BaseCurrency, u.CreatedAt.Format("2006-01-02"))
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
