package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yuluo-yx/mory/appcli"
)

type desktopClient interface {
	Open(context.Context, string) error
	Export(context.Context, appcli.ExportRequest) error
}

func newRootCommand() *cobra.Command {
	return newRootCommandWithClient(func(appPath string) desktopClient { return appcli.Client{AppPath: appPath} })
}

func newRootCommandWithClient(clientFor func(string) desktopClient) *cobra.Command {
	var appPath string
	root := &cobra.Command{
		Use:           "mory <document>",
		Short:         "Open and export Markdown documents with Mory",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			document, err := appcli.ResolveDocument(args[0])
			if err != nil {
				return err
			}
			return clientFor(appPath).Open(cmd.Context(), document)
		},
	}
	root.PersistentFlags().StringVar(&appPath, "app", "", "path to Mory.app or Mory.exe")
	root.AddCommand(newExportCommand(&appPath, clientFor))
	return root
}

func newExportCommand(appPath *string, clientFor func(string) desktopClient) *cobra.Command {
	var format string
	var outputDirectory string
	var force bool
	command := &cobra.Command{
		Use:     "export <document>",
		Short:   "Export a document through the native Mory renderer",
		Example: "  mory export --format=pdf --path=./ guide.md",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := appcli.ResolveExport(args[0], format, outputDirectory, force)
			if err != nil {
				return err
			}
			if err := clientFor(*appPath).Export(cmd.Context(), request); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), request.Destination)
			return err
		},
	}
	command.Flags().StringVar(&format, "format", "pdf", "output format: html, pdf, png, jpeg, or pptx")
	command.Flags().StringVar(&outputDirectory, "path", ".", "output directory")
	command.Flags().BoolVarP(&force, "force", "f", false, "replace an existing output file")
	return command
}
