package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	model2 "ledger/internal/model"
	"ledger/internal/util"
	"ledger/internal/wiring"
	"net/mail"
	"os"
	"strings"

	"github.com/aarondl/opt/omit"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
	"github.com/stephenafamo/bob"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 72
)

var makeCmd = &cobra.Command{
	Use:   "make",
	Short: "Creates various items such as users",
}

func init() {
	makeCmd.AddCommand(makeUserCmd)

	rootCmd.AddCommand(makeCmd)
}

func readLine(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)

	line, err := reader.ReadString('\n')

	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", err
	}

	return strings.TrimSpace(line), nil
}

func readSecret(reader *bufio.Reader, prompt string) ([]byte, error) {
	fmt.Print(prompt)

	fd := int(os.Stdin.Fd())

	if reader.Buffered() == 0 && term.IsTerminal(fd) {
		secret, err := term.ReadPassword(fd)
		fmt.Println()

		if err != nil {
			return nil, err
		}

		return secret, nil
	}

	line, err := reader.ReadString('\n')
	fmt.Println()

	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return nil, err
	}

	return []byte(strings.TrimRight(line, "\r\n")), nil
}

func normalizeEmail(email string) (string, error) {
	if email == "" {
		return "", errors.New("email must not be empty")
	}

	address, err := mail.ParseAddress(email)

	if err != nil {
		return "", fmt.Errorf("invalid email address: %w", err)
	}

	return strings.ToLower(address.Address), nil
}

func validatePassword(password []byte) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("password must be at least %d bytes long", minPasswordLength)
	}

	if len(password) > maxPasswordLength {
		return fmt.Errorf("password must be at most %d bytes long", maxPasswordLength)
	}

	return nil
}

var makeUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Creates a new user",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withApp(cmd.Context(), wiring.Database(wiring.Config()),
			func(ctx context.Context, i do.Injector) error {
				db, err := do.Invoke[bob.DB](i)
				if err != nil {
					return err
				}

				return makeUser(ctx, db)
			})
	},
}

func makeUser(ctx context.Context, db bob.DB) error {
	reader := bufio.NewReader(os.Stdin)

	username, err := readLine(reader, "Username: ")
	if err != nil {
		return err
	}

	if username == "" {
		return errors.New("username must not be empty")
	}

	rawEmail, err := readLine(reader, "Email: ")
	if err != nil {
		return err
	}

	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return err
	}

	emailExists, err := model2.Users.Query(
		model2.SelectWhere.Users.Email.EQ(email),
	).Exists(ctx, db)

	if err != nil {
		return err
	}

	if emailExists {
		return errors.New("user with that email already exists")
	}

	password, err := readSecret(reader, "Password: ")
	if err != nil {
		return err
	}

	if err := validatePassword(password); err != nil {
		return err
	}

	confirmPassword, err := readSecret(reader, "Confirm password: ")
	if err != nil {
		return err
	}

	if !bytes.Equal(password, confirmPassword) {
		return errors.New("passwords do not match")
	}

	passwordHash, err := util.HashPassword(password)

	if err != nil {
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			return fmt.Errorf("password must be at most %d bytes long", maxPasswordLength)
		}

		return err
	}

	user, err := model2.Users.Insert(&model2.UserSetter{
		Name:         omit.From(username),
		Email:        omit.From(email),
		PasswordHash: omit.From(passwordHash),
	}).One(ctx, db)

	if err != nil {
		return err
	}

	fmt.Printf("Created user %s with id %d\n", user.Name, user.ID)

	return nil
}
