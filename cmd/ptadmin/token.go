package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func runToken(ctx context.Context, conn *db.DB, sub string, args []string) int {
	switch sub {
	case "create":
		return tokenCreate(ctx, conn, args)
	case "list":
		return tokenList(ctx, conn, args)
	case "revoke":
		return tokenRevoke(ctx, conn, args)
	default:
		return errf("token: unknown subcommand %q", sub)
	}
}

func tokenCreate(ctx context.Context, conn *db.DB, args []string) int {
	fs := flag.NewFlagSet("token create", flag.ContinueOnError)
	userEmail := fs.String("user", "", "user email (required)")
	name := fs.String("name", "", "token name (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *userEmail == "" || *name == "" {
		return errf("token create: --user and --name are required")
	}
	u, err := conn.GetUserByEmail(ctx, *userEmail)
	if errors.Is(err, db.ErrNotFound) {
		return errf("token create: no such user %q", *userEmail)
	}
	if err != nil {
		return errf("token create: %v", err)
	}

	plain, hash, err := auth.GenerateToken()
	if err != nil {
		return errf("token create: %v", err)
	}
	tok := &domain.Token{UserID: u.ID, Name: *name, Hash: hash}
	if err := conn.CreateToken(ctx, tok); err != nil {
		return errf("token create: %v", err)
	}
	fmt.Println("Token (store this, it will not be shown again):")
	fmt.Println(plain)
	fmt.Printf("\nid=%d user=%s name=%s\n", tok.ID, u.Email, tok.Name)
	return 0
}

func tokenList(ctx context.Context, conn *db.DB, args []string) int {
	fs := flag.NewFlagSet("token list", flag.ContinueOnError)
	userEmail := fs.String("user", "", "filter by user email")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var rows []*domain.Token
	if *userEmail != "" {
		u, err := conn.GetUserByEmail(ctx, *userEmail)
		if err != nil {
			return errf("token list: %v", err)
		}
		rows, err = conn.ListTokens(ctx, u.ID)
		if err != nil {
			return errf("token list: %v", err)
		}
	} else {
		users, err := conn.ListUsers(ctx)
		if err != nil {
			return errf("token list: %v", err)
		}
		for _, u := range users {
			r, err := conn.ListTokens(ctx, u.ID)
			if err != nil {
				return errf("token list: %v", err)
			}
			rows = append(rows, r...)
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tUSER\tNAME\tCREATED\tLAST USED\tREVOKED")
	for _, t := range rows {
		_, _ = fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%s\t%s\n",
			t.ID, t.UserID, t.Name,
			t.CreatedAt.Format("2006-01-02"),
			fmtOptTime(t.LastUsedAt),
			fmtOptTime(t.RevokedAt),
		)
	}
	_ = w.Flush()
	return 0
}

func tokenRevoke(ctx context.Context, conn *db.DB, args []string) int {
	fs := flag.NewFlagSet("token revoke", flag.ContinueOnError)
	id := fs.Int64("id", 0, "token id (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == 0 {
		return errf("token revoke: --id is required")
	}
	if err := conn.RevokeToken(ctx, *id); err != nil {
		return errf("token revoke: %v", err)
	}
	fmt.Printf("token revoked: id=%d\n", *id)
	return 0
}

func fmtOptTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02")
}
