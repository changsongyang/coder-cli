package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"cdr.dev/coder-cli/internal/entclient"
	"cdr.dev/coder-cli/internal/x/xtabwriter"
	"github.com/manifoldco/promptui"
	"github.com/urfave/cli"
	"golang.org/x/xerrors"

	"go.coder.com/flog"
)

func makeSecretsCmd() cli.Command {
	return cli.Command{
		Name:        "secrets",
		Usage:       "Interact with Coder Secrets",
		Description: "Interact with secrets objects owned by the active user.",
		Action:      exitHelp,
		Subcommands: []cli.Command{
			{
				Name:   "ls",
				Usage:  "List all secrets owned by the active user",
				Action: listSecrets,
			},
			makeCreateSecret(),
			{
				Name:      "rm",
				Usage:     "Remove one or more secrets by name",
				ArgsUsage: "[...secret_name]",
				Action:    removeSecrets,
			},
			{
				Name:      "view",
				Usage:     "View a secret by name",
				ArgsUsage: "[secret_name]",
				Action:    viewSecret,
			},
		},
	}
}

func makeCreateSecret() cli.Command {
	var (
		fromFile    string
		fromLiteral string
		fromPrompt  bool
		description string
	)

	return cli.Command{
		Name:        "create",
		Usage:       "Create a new secret",
		Description: "Create a new secret object to store application secrets and access them securely from within your environments.",
		ArgsUsage:   "[secret_name]",
		Before: func(c *cli.Context) error {
			if c.Args().First() == "" {
				return xerrors.Errorf("[secret_name] is a required argument")
			}
			if fromPrompt && (fromLiteral != "" || fromFile != "") {
				return xerrors.Errorf("--from-prompt cannot be set along with --from-file or --from-literal")
			}
			if fromLiteral != "" && fromFile != "" {
				return xerrors.Errorf("--from-literal and --from-file cannot both be set")
			}
			if !fromPrompt && fromFile == "" && fromLiteral == "" {
				return xerrors.Errorf("one of [--from-literal, --from-file, --from-prompt] is required")
			}
			return nil
		},
		Action: func(c *cli.Context) {
			var (
				client = requireAuth()
				name   = c.Args().First()
				value  string
				err    error
			)
			if fromLiteral != "" {
				value = fromLiteral
			} else if fromFile != "" {
				contents, err := ioutil.ReadFile(fromFile)
				requireSuccess(err, "failed to read file: %v", err)
				value = string(contents)
			} else {
				prompt := promptui.Prompt{
					Label: "value",
					Mask:  '*',
					Validate: func(s string) error {
						if len(s) < 1 {
							return xerrors.Errorf("a length > 0 is required")
						}
						return nil
					},
				}
				value, err = prompt.Run()
				requireSuccess(err, "failed to prompt for value: %v", err)
			}

			err = client.InsertSecret(entclient.InsertSecretReq{
				Name:        name,
				Value:       value,
				Description: description,
			})
			requireSuccess(err, "failed to insert secret: %v", err)
		},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:        "from-file",
				Usage:       "a file from which to read the value of the secret",
				TakesFile:   true,
				Destination: &fromFile,
			},
			cli.StringFlag{
				Name:        "from-literal",
				Usage:       "the value of the secret",
				Destination: &fromLiteral,
			},
			cli.BoolFlag{
				Name:        "from-prompt",
				Usage:       "enter the secret value through a terminal prompt",
				Destination: &fromPrompt,
			},
			cli.StringFlag{
				Name:        "description",
				Usage:       "a description of the secret",
				Destination: &description,
			},
		},
	}
}

func listSecrets(_ *cli.Context) {
	client := requireAuth()

	secrets, err := client.Secrets()
	requireSuccess(err, "failed to get secrets: %v", err)

	if len(secrets) < 1 {
		flog.Info("No secrets found")
		return
	}

	err = xtabwriter.WriteTable(len(secrets), func(i int) interface{} {
		s := secrets[i]
		s.Value = "******" // value is omitted from bulk responses
		return s
	})
	requireSuccess(err, "failed to write table of secrets: %w", err)
}

func viewSecret(c *cli.Context) {
	var (
		client = requireAuth()
		name   = c.Args().First()
	)
	if name == "" {
		flog.Fatal("[name] is a required argument")
	}

	secret, err := client.SecretByName(name)
	requireSuccess(err, "failed to get secret by name: %v", err)

	_, err = fmt.Fprintln(os.Stdout, secret.Value)
	requireSuccess(err, "failed to write: %v", err)
}

func removeSecrets(c *cli.Context) {
	var (
		client = requireAuth()
		names  = append([]string{c.Args().First()}, c.Args().Tail()...)
	)
	if len(names) < 1 || names[0] == "" {
		flog.Fatal("[...secret_name] is a required argument")
	}

	errorSeen := false
	for _, n := range names {
		err := client.DeleteSecretByName(n)
		if err != nil {
			flog.Error("failed to delete secret: %v", err)
			errorSeen = true
		} else {
			flog.Info("Successfully deleted secret %q", n)
		}
	}
	if errorSeen {
		os.Exit(1)
	}
}
